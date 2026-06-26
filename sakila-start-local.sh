#!/usr/bin/env bash

# This script starts local Postgres, MySQL, SQL Server, ClickHouse,
# Oracle, and rqlite (via the corresponding sakiladb/* docker images)
# for repo-wide integration tests.
#
# Each `docker run` uses `--pull always` so a republished image (same tag, new
# digest) is fetched before the container starts. Without it, a locally-cached
# image can be silently stale, e.g. an old sakiladb/sqlserver:2019 with 5 views
# instead of the current 7, which makes the sakila count tests fail confusingly.
# NOTE: This script has only been tested on MacOS on Apple Silicon.

set +e
# First, kill any already running services.
./sakila-stop-local.sh &>/dev/null

set -e

docker run -d --pull always -p 5432:5432 --name sakiladb-pg sakiladb/postgres:12 &>/dev/null
docker run -d --pull always -p 3306:3306 --name sakiladb-my sakiladb/mysql:8 &>/dev/null
docker run -d --pull always -p 9000:9000 --name sakiladb-ch sakiladb/clickhouse:25 &>/dev/null
docker run -d --pull always -p 1521:1521 --name sakiladb-or sakiladb/oracle:23 &>/dev/null
docker run -d --pull always -p 1433:1433 --name sakiladb-ms --platform=linux/amd64 sakiladb/sqlserver:2019 &>/dev/null
docker run -d --pull always -p 4001:4001 --name sakiladb-rq sakiladb/rqlite:10 &>/dev/null

sleep 5

# Print the envars that need to be exported for the sq e2e tests to work
# correctly.

echo "Export these envars (and source them) to run the tests with these sources enabled"

cat << EOF
export SQ_TEST_SRC__SAKILA_PG12="postgres://sakila:p_ssW0rd@localhost:5432/sakila"
export SQ_TEST_SRC__SAKILA_MS19="sqlserver://sakila:p_ssW0rd@localhost:1433?database=sakila"
export SQ_TEST_SRC__SAKILA_MY8="mysql://sakila:p_ssW0rd@localhost:3306/sakila"
export SQ_TEST_SRC__SAKILA_CH25="clickhouse://sakila:p_ssW0rd@localhost:9000?database=sakila"
export SQ_TEST_SRC__SAKILA_OR23="oracle://sakila:p_ssW0rd@localhost:1521/SAKILA"
export SQ_TEST_SRC__SAKILA_RQ10="rqlite://sakila:p_ssW0rd@localhost:4001?disableClusterDiscovery=true"
EOF
export SQ_TEST_SRC__SAKILA_PG12="postgres://sakila:p_ssW0rd@localhost:5432/sakila"
export SQ_TEST_SRC__SAKILA_MS19="sqlserver://sakila:p_ssW0rd@localhost:1433?database=sakila"
export SQ_TEST_SRC__SAKILA_MY8="mysql://sakila:p_ssW0rd@localhost:3306/sakila"
export SQ_TEST_SRC__SAKILA_CH25="clickhouse://sakila:p_ssW0rd@localhost:9000?database=sakila"
export SQ_TEST_SRC__SAKILA_OR23="oracle://sakila:p_ssW0rd@localhost:1521/SAKILA"
export SQ_TEST_SRC__SAKILA_RQ10="rqlite://sakila:p_ssW0rd@localhost:4001?disableClusterDiscovery=true"
