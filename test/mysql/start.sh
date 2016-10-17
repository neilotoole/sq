#!/usr/bin/env bash

docker run -p 33067:3306 --restart=unless-stopped --name sq-mysql -e MYSQL_ROOT_PASSWORD=root -e MYSQL_DATABASE=sq_mydb1 -d mysql:5.7