package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/vk26/social-network/models"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/context"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	Router  *mux.Router
	DB      *sql.DB
	DBSlave *sql.DB
	Tmpl    *template.Template
}

var (
	sessionKey   = []byte(os.Getenv("SOCIAL_APP_SESSIONS_KEY"))
	sessionStore = sessions.NewCookieStore(sessionKey)
)

type ctxKey string

const (
	currentUserKey ctxKey = "currentUserKey"
)

func init() {
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   3600 * 24 * 15, // 15 days
		HttpOnly: true,
	}
}

func main() {
	a := App{}
	a.Initialize(
		"mysql",
		os.Getenv("SOCIAL_APP_MYSQL_DSN"),
		os.Getenv("SOCIAL_APP_MYSQL_DSN_SLAVE"),
	)
	port := os.Getenv("PORT")
	fmt.Println("App is listening port ", port)
	a.Run(":" + port)
}

func (a *App) Initialize(dbDriver, dsn string, dsnSlave string) {
	var err error

	a.DB, err = sql.Open(dbDriver, dsn)
	a.DB.SetMaxOpenConns(150)
	a.DB.SetMaxIdleConns(100)
	a.DB.SetConnMaxLifetime(time.Minute * 2)

	a.DBSlave, err = sql.Open(dbDriver, dsnSlave)
	a.DBSlave.SetMaxOpenConns(150)
	a.DBSlave.SetMaxIdleConns(100)
	a.DBSlave.SetConnMaxLifetime(time.Minute * 2)

	if err != nil {
		log.Fatal(err)
	}

	a.Router = mux.NewRouter()
	a.Tmpl = template.Must(template.ParseGlob("frontend/templates/*"))
	a.initializeRoutes()
}

func (a *App) Run(addr string) {
	srv := &http.Server{
		Addr:         addr,
		Handler:      a.Router,
		ReadTimeout:  time.Second * 15,
		WriteTimeout: time.Second * 5,
	}
	log.Fatal(srv.ListenAndServe())
}

func (a *App) initializeRoutes() {
	siteRouter := mux.NewRouter().PathPrefix("/").Subrouter()
	siteRouter.HandleFunc("/login", a.LoginForm).Methods("GET")
	siteRouter.HandleFunc("/login", a.Login).Methods("POST")
	siteRouter.HandleFunc("/logout", a.Logout).Methods("GET")
	siteRouter.HandleFunc("/signup", a.SignupForm).Methods("GET")
	siteRouter.HandleFunc("/signup", a.Signup).Methods("POST")
	siteRouter.HandleFunc("/users", a.UsersList).Methods("GET")
	siteRouter.HandleFunc("/users/search", a.UsersSearch).Queries("name_substr", "{name_substr}").Methods("GET")
	siteRouter.HandleFunc("/", a.Home).Methods("GET")

	authRouter := siteRouter.PathPrefix("/").Subrouter()
	authRouter.HandleFunc("/users/{id:[0-9]+}", a.UserPage).Methods("GET")
	authRouter.Use(a.authMiddleware)

	siteRouter.Use(a.getCurrentUserMiddleware)

	assetsHandler := http.StripPrefix("/data/", http.FileServer(http.Dir("frontend/assets")))
	siteRouter.PathPrefix("/data/").Handler(assetsHandler)

	a.Router = siteRouter
}

func (a *App) LoginForm(w http.ResponseWriter, r *http.Request) {
	err := a.Tmpl.ExecuteTemplate(w, "login.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *App) Login(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	user := models.User{Email: email}
	user.GetUserByEmail(a.DB)
	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
	}

	session, _ := sessionStore.Get(r, "social_app")
	session.Values["userID"] = user.Id
	session.Save(r, w)

	userIDStr := strconv.FormatInt(int64(user.Id), 10)
	http.Redirect(w, r, "/users/"+userIDStr, http.StatusFound)
}

func (a *App) Logout(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "social_app")
	session.Values["userID"] = nil
	session.Save(r, w)

	http.Redirect(w, r, "/login", http.StatusFound)
}

func (a *App) SignupForm(w http.ResponseWriter, r *http.Request) {
	a.Tmpl.ExecuteTemplate(w, "signup.html", nil)
}

func (a *App) Signup(w http.ResponseWriter, r *http.Request) {
	passwordHash, _ := bcrypt.GenerateFromPassword([]byte(r.FormValue("password")), 14)
	user := models.User{
		Name:         r.FormValue("name"),
		Surname:      r.FormValue("surname"),
		Birthday:     r.FormValue("birthday"),
		City:         r.FormValue("city"),
		About:        r.FormValue("about"),
		Email:        r.FormValue("email"),
		PasswordHash: string(passwordHash),
	}
	err := user.CreateUser(a.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session, _ := sessionStore.Get(r, "social_app")
	session.Values["userID"] = int(user.Id)
	session.Save(r, w)

	userIDStr := strconv.FormatInt(int64(user.Id), 10)
	http.Redirect(w, r, "/users/"+userIDStr, http.StatusFound)

}

func (a *App) Home(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (a *App) UserPage(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(mux.Vars(r)["id"])
	user := models.User{Id: id}
	err := user.GetUserByID(a.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"user":        user,
		"currentUser": context.Get(r, currentUserKey),
	}
	a.Tmpl.ExecuteTemplate(w, "user_page.html", data)
}

func (a *App) UsersList(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	page, _ := strconv.Atoi(params["page"])
	count, _ := strconv.Atoi(params["count"])
	if count == 0 {
		count = 15
	}
	start := page * count
	users, err := models.GetUsers(a.DBSlave, count, start)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"users":       users,
		"currentUser": context.Get(r, currentUserKey),
	}
	a.Tmpl.ExecuteTemplate(w, "users_list.html", data)
}

func (a *App) UsersSearch(w http.ResponseWriter, r *http.Request) {
	nameSubstr := r.FormValue("name_substr")
	page, _ := strconv.Atoi(r.FormValue("page"))
	count, _ := strconv.Atoi(r.FormValue("count"))
	if count == 0 {
		count = 15
	}
	start := page * count
	users, err := models.SearchUsers(a.DBSlave, nameSubstr, count, start)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := map[string]interface{}{
		"users":       users,
		"currentUser": context.Get(r, currentUserKey),
	}
	a.Tmpl.ExecuteTemplate(w, "users_list.html", data)
}

func (a *App) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Authentication check ...")
		session, _ := sessionStore.Get(r, "social_app")
		userID, ok := session.Values["userID"].(int)
		if !ok || userID == 0 {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *App) getCurrentUserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := sessionStore.Get(r, "social_app")
		userID, ok := session.Values["userID"].(int)
		if ok && userID != 0 {
			user := models.User{Id: userID}
			err := user.GetUserByID(a.DB)
			if err == nil {
				context.Set(r, currentUserKey, user)
			}
		}
		next.ServeHTTP(w, r)
	})
}
