#!/bin/ash

cd /tmp/test_db
mariadb -u root -p$MYSQL_PASSWORD < employees.sql
mariadb --user=root --password=$MYSQL_PASSWORD --database=employees --execute="GRANT ALL on employees.* TO 'mysql'@'%';"
mariadb --user=root --password=$MYSQL_PASSWORD --database=employees --execute="FLUSH PRIVILEGES;"
