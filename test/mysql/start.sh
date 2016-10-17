#!/usr/bin/env bash

docker run -p 3306:3306 -v `pwd`/sqtest1.sql:/docker-entrypoint-initdb.d/sqtest1.sql --restart=unless-stopped --name sq-mysql -e MYSQL_ROOT_PASSWORD=root -e MYSQL_USER=sq -e MYSQL_PASSWORD=sq -e MYSQL_DATABASE=sqtest1 -d mysql:5.7