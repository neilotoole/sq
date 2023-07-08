#!/usr/bin/env bash

# This script starts local versions of Postgres, MySQL,
# and Azure SQL Edge. We use ASE instead of SQL Server, because
# ASE works on both arm and amd.
#
# Use:
#
#  $ source sakila-start-local.sh

set +e
# First, kill any already running services.
./sakila-stop-local.sh &>/dev/null

set -e

docker run -d -p 5432:5432 --name sakiladb-pg sakiladb/postgres:12 &>/dev/null
docker run -d -p 1433:1433 --name sakiladb-az sakiladb/azure-sql-edge:1.0.7 &>/dev/null
docker run -d -p 3306:3306 --name sakiladb-my sakiladb/mysql:8 &>/dev/null

sleep 5

cat << EOF
export SQ_TEST_SRC__SAKILA_PG12=localhost
export SQ_TEST_SRC__SAKILA_AZ1=localhost
export SQ_TEST_SRC__SAKILA_MY8=localhost
EOF
export SQ_TEST_SRC__SAKILA_PG12=localhost
export SQ_TEST_SRC__SAKILA_AZ1=localhost
export SQ_TEST_SRC__SAKILA_MY8=localhost
