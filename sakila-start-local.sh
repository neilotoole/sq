#!/usr/bin/env bash

# This script starts local versions of Postgres, MySQL, and SQL Server.
# NOTE: This script has only been tested on MacOS on Apple Silicon.
#
# Use:
#
#  $ source sakila-start-local.sh

set +e
# First, kill any already running services.
./sakila-stop-local.sh &>/dev/null

set -e

docker run -d -p 5432:5432 --name sakiladb-pg sakiladb/postgres:12 &>/dev/null
docker run --platform=linux/amd64 -d -p 1433:1433 --name sakiladb-ms sakiladb/sqlserver:2019 &>/dev/null
docker run -d -p 3306:3306 --name sakiladb-my sakiladb/mysql:8 &>/dev/null

sleep 5

cat << EOF
export SQ_TEST_SRC__SAKILA_PG12=localhost
export SQ_TEST_SRC__SAKILA_MS19=localhost
export SQ_TEST_SRC__SAKILA_MY8=localhost
EOF
export SQ_TEST_SRC__SAKILA_PG12=localhost
export SQ_TEST_SRC__SAKILA_MS19=localhost
export SQ_TEST_SRC__SAKILA_MY8=localhost
