# About
This project is homework project of course - [Highload Architect by OTUS](https://otus.ru/lessons/arhitektor-vysokih-nagruzok/)

https://warm-chamber-70708.herokuapp.com/

## HW1 Social-Network MVP
Write MVP of social network with user's pages. Don't use ORM, indexes, patterns of Highload/High availability

## HW2 Using indexes
Write searching users with `sql LIKE %` condition. Select an efficient index, inspect via `EXPALIN`. Do load testing before and after using indexes.

[Report](/reports/hw2_indexes/readme.md)

## HW3 Setup master/slave replication
Setup replication master/slave. Redirect some requests to slave-node. Do load testing before and after separation requests by master/slave.

[Report](/reports/hw3_master_slave_replication/readme.md)

## HW4 Promote slave to master
Add new slave2. Setup ROW-based replication. Turn on GTID. Config semi-sync replication. Start DB seed application with counting success insertions. Kill master. Promote slave1 to master. Switch replication slave2 from slave1(new master). Check that we didn't lose transactions.

[Report](/reports/hw4_switch_master/readme.md)