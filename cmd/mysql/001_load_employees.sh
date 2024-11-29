#!/bin/ash

cd /tmp/test_db
mysql -u root -p $MYSQL_PASSWORD < employees.sql
echo GRANT ALL ON employees.* To \'$MYSQL_USER\'@\'%\' | mysql -U root -p $MYSQL_PASSWORD -e