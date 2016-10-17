#!/usr/bin/env bash

docker run -p 5432:5432 -v `pwd`/sqtest1.sql:/docker-entrypoint-initdb.d/sqtest1.sql --restart=unless-stopped --name sq-postgres -e POSTGRES_USER=sq -e POSTGRES_PASSWORD=sq -e POSTGRES_DB=sqtest1 -d postgres:9.5

# to connect using psql:
# psql -h localhost -d pqdb1 -U sq