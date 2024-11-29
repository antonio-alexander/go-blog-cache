#!/bin/ash

cd /tmp/test_db
mysql -u root -p$MYSQL_PASSWORD < employees.sql
mysql --user=root --password=$MYSQL_PASSWORD --database=employees --execute="GRANT ALL on employees.* TO 'mysql'@'%';"
mysql --user=root --password=$MYSQL_PASSWORD --database=employees --execute="FLUSH PRIVILEGES;"
