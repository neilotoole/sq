#!/usr/bin/env bash

# Stops the containers started by sakila-start-local.sh.
#
# Usage:
#
#   ./sakila-stop-local.sh                  # every engine
#   ./sakila-stop-local.sh postgres mysql   # just these
#
# Engine names are the keys of .github/sakila-db.json.

set -uo pipefail

# Keep in sync with sakila-start-local.sh.
declare -A cname=(
  [postgres]=sakiladb-pg [mysql]=sakiladb-my [sqlserver]=sakiladb-ms
  [clickhouse]=sakiladb-ch [oracle]=sakiladb-or [rqlite]=sakiladb-rq
)

engines=("$@")
if [ ${#engines[@]} -eq 0 ]; then
  engines=(postgres mysql sqlserver clickhouse oracle rqlite)
fi

for engine in "${engines[@]}"; do
  if [ -z "${cname[$engine]:-}" ]; then
    echo "unknown engine: $engine" >&2
    exit 2
  fi
done

for engine in "${engines[@]}"; do
  docker rm -f "${cname[$engine]}"
done
