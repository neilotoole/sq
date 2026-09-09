#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
config="$here/../.github/sakila-db.json"

# Keep the default-registry assertions hermetic: a SAKILADB_REGISTRY exported in
# the caller's shell would otherwise override the default and fail them. Cases
# that need an override set it inline (prefixed on a single invocation).
unset SAKILADB_REGISTRY

build() { "$here/build-db-matrix.sh" "$@"; }

# explicit tag: one entry carrying the engine's port and env
out=$(build '{"postgres":["12"]}')
echo "$out" | jq -e 'length == 1' >/dev/null
echo "$out" | jq -e '.[0].engine == "postgres"' >/dev/null
echo "$out" | jq -e '.[0].tag == "12"' >/dev/null
echo "$out" | jq -e '.[0].port == 5432' >/dev/null
echo "$out" | jq -e '.[0].env == "SQ_TEST_SRC__SAKILA_PG"' >/dev/null

# the image ref defaults to GHCR and is stamped once here for downstream reuse
echo "$out" | jq -e '.[0].image == "ghcr.io/sakiladb/postgres:12"' >/dev/null

# no scope, no packages: every leg runs go test ./... (gh #1133)
echo "$out" | jq -e 'all(has("packages") | not)' >/dev/null

# the DSN must NOT travel through the matrix: GitHub masks the credential and
# drops a job output containing it, which silently empties the matrix.
echo "$out" | jq -e 'all(has("dsn") | not)' >/dev/null

# explicit tags pass through in the order given
out=$(build '{"mysql":["8","9"]}')
echo "$out" | jq -e 'length == 2' >/dev/null
echo "$out" | jq -e '[.[].tag] == ["8","9"]' >/dev/null
echo "$out" | jq -e '[.[].image] == ["ghcr.io/sakiladb/mysql:8","ghcr.io/sakiladb/mysql:9"]' >/dev/null

# an explicit numeric tag not (yet) in the tags list still passes through, so a
# new image can be tried before it is added to sakila-db.json
out=$(build '{"postgres":["19"]}')
echo "$out" | jq -e '[.[].tag] == ["19"]' >/dev/null

# SAKILADB_REGISTRY overrides the image registry (single source of truth)
out=$(SAKILADB_REGISTRY=example.test/ns build '{"postgres":["12"]}')
echo "$out" | jq -e '.[0].image == "example.test/ns/postgres:12"' >/dev/null

# a trailing slash in the override is tolerated (not doubled into an invalid ref)
out=$(SAKILADB_REGISTRY=example.test/ns/ build '{"postgres":["12"]}')
echo "$out" | jq -e '.[0].image == "example.test/ns/postgres:12"' >/dev/null

# oldest: the last entry in the engine's tags (tags are newest-first)
out=$(build '{"postgres":["oldest"]}')
echo "$out" | jq -e '[.[].tag] == ["9"]' >/dev/null

# bookends: latest then oldest, in that order
out=$(build '{"postgres":["bookends"]}')
echo "$out" | jq -e '[.[].tag] == ["latest","9"]' >/dev/null

# bookends on a single-tag engine yields latest and that tag; collapsing the
# pair to one leg is dedup-db-matrix.sh's job (see dedup-db-matrix_test.sh)
out=$(build '{"clickhouse":["bookends"]}')
echo "$out" | jq -e '[.[].tag] == ["latest","25"]' >/dev/null

# all: every entry in the engine's tags, in list order
out=$(build '{"mysql":["all"]}')
want=$(jq -c '.mysql.tags' "$config")
echo "$out" | jq -e --argjson want "$want" '[.[].tag] == $want' >/dev/null

# "*": every engine; "all" across every engine is one entry per tag
out=$(build '{"*":["all"]}')
total=$(jq '[.[].tags | length] | add' "$config")
echo "$out" | jq -e --argjson n "$total" 'length == $n' >/dev/null

# "*" plus an explicit engine appends to that engine's list
out=$(build '{"*":["latest"],"postgres":["9"]}')
engines=$(jq 'keys | length' "$config")
echo "$out" | jq -e --argjson n "$engines" 'length == $n + 1' >/dev/null
echo "$out" | jq -e '[.[] | select(.engine == "postgres") | .tag] == ["latest","9"]' >/dev/null

# exact-string duplicates are dropped, first occurrence wins
out=$(build '{"postgres":["bookends","oldest","9","latest"]}')
echo "$out" | jq -e '[.[].tag] == ["latest","9"]' >/dev/null

# unknown engine is a hard error, not a silently-empty/null row
if build '{"bogus":["1"]}' 2>/dev/null; then
  echo "build-db-matrix_test: FAIL (expected error for unknown engine)" >&2
  exit 1
fi

# unknown selector is a hard error (a typo must not silently select nothing)
if build '{"postgres":["bookend"]}' 2>/dev/null; then
  echo "build-db-matrix_test: FAIL (expected error for unknown selector)" >&2
  exit 1
fi

# contract: every engine's tags list is newest-first. oldest/bookends depend on
# it. Compared numerically per dotted component so 5.7 sorts below 9.
jq -e '
  to_entries | all(
    .value.tags
    | map(split(".") | map(tonumber))
    | . == (sort | reverse))
' "$config" >/dev/null

echo "build-db-matrix_test: PASS"
