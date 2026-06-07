#!/usr/bin/env bash

# This script starts local Postgres, MySQL, SQL Server, ClickHouse,
# Oracle, and rqlite (via the corresponding sakiladb/* docker images)
# for repo-wide integration tests.
# NOTE: This script has only been tested on MacOS on Apple Silicon.

set +e
# First, kill any already running services.
./sakila-stop-local.sh &>/dev/null

set -e

docker run -d -p 5432:5432 --name sakiladb-pg sakiladb/postgres:12 &>/dev/null
docker run -d -p 3306:3306 --name sakiladb-my sakiladb/mysql:8 &>/dev/null
docker run -d -p 9000:9000 --name sakiladb-ch sakiladb/clickhouse:25 &>/dev/null
docker run -d -p 1521:1521 --name sakiladb-or sakiladb/oracle:23 &>/dev/null
docker run -d -p 1433:1433 --name sakiladb-ms --platform=linux/amd64 sakiladb/sqlserver:2019 &>/dev/null
# rqlite needs --add-host rqlite1:127.0.0.1 because its Raft state
# advertises as rqlite1; without it the node fails to bootstrap.
docker run -d -p 4001:4001 --add-host rqlite1:127.0.0.1 --name sakiladb-rq sakiladb/rqlite:10 &>/dev/null

sleep 5

# Print the envars that need to be exported for the sq e2e tests to work
# correctly.

echo "Export these envars (and source them) to run the tests with these sources enabled"

cat << EOF
export SQ_TEST_SRC__SAKILA_PG12=localhost
export SQ_TEST_SRC__SAKILA_MS19=localhost
export SQ_TEST_SRC__SAKILA_MY8=localhost
export SQ_TEST_SRC__SAKILA_CH25=localhost
export SQ_TEST_SRC__SAKILA_OR23=localhost
export SQ_TEST_SRC__SAKILA_RQ=localhost:4001
EOF
export SQ_TEST_SRC__SAKILA_PG12=localhost
export SQ_TEST_SRC__SAKILA_MS19=localhost
export SQ_TEST_SRC__SAKILA_MY8=localhost
export SQ_TEST_SRC__SAKILA_CH25=localhost
export SQ_TEST_SRC__SAKILA_OR23=localhost
export SQ_TEST_SRC__SAKILA_RQ=localhost:4001
