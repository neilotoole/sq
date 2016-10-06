#!/usr/bin/env bash

docker run -p 5432:5432 --restart=unless-stopped --name sq-pq -e POSTGRES_USER=sq -e POSTGRES_PASSWORD=sq -e POSTGRES_DB=pqdb1 -d postgres:9.5

# to connect using psql:
# psql -h localhost -d pqdb1 -U sq