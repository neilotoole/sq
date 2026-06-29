#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"

# narrow scope: postgres@12 picks per-engine packages
out=$("$here/build-db-matrix.sh" narrow '{"postgres":["12"]}')
echo "$out" | jq -e 'length == 1' >/dev/null
echo "$out" | jq -e '.[0].engine == "postgres"' >/dev/null
echo "$out" | jq -e '.[0].tag == "12"' >/dev/null
echo "$out" | jq -e '.[0].port == 5432' >/dev/null
echo "$out" | jq -e '.[0].env == "SQ_TEST_SRC__SAKILA_PG"' >/dev/null
echo "$out" | jq -e '.[0].packages | test("drivers/postgres")' >/dev/null

# full scope: packages collapses to ./...
out=$("$here/build-db-matrix.sh" full '{"mysql":["8","9"]}')
echo "$out" | jq -e 'length == 2' >/dev/null
echo "$out" | jq -e 'all(.packages == "./...")' >/dev/null
echo "$out" | jq -e '[.[].tag] == ["8","9"]' >/dev/null

echo "build-db-matrix_test: PASS"
