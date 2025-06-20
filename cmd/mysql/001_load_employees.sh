#!/bin/ash

alias mysql=mariadb

cd /tmp/test_db

mysql --user=root --password=$MYSQL_ROOT_PASSWORD < employees.sql
mysql --user=root --password=$MYSQL_ROOT_PASSWORD --database=employees --execute="GRANT ALL on employees.* TO 'mysql'@'%';"
mysql --user=root --password=$MYSQL_ROOT_PASSWORD --database=employees --execute="FLUSH PRIVILEGES;"

unalias mysql