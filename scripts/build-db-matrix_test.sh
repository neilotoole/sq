#!/usr/bin/env bash
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"

# Keep the default-registry assertions hermetic: a SAKILADB_REGISTRY exported in
# the caller's shell would otherwise override the default and fail them. Cases
# that need an override set it inline (prefixed on a single invocation).
unset SAKILADB_REGISTRY

# narrow scope: postgres@12 picks per-engine packages
out=$("$here/build-db-matrix.sh" narrow '{"postgres":["12"]}')
echo "$out" | jq -e 'length == 1' >/dev/null
echo "$out" | jq -e '.[0].engine == "postgres"' >/dev/null
echo "$out" | jq -e '.[0].tag == "12"' >/dev/null
echo "$out" | jq -e '.[0].port == 5432' >/dev/null
echo "$out" | jq -e '.[0].env == "SQ_TEST_SRC__SAKILA_PG"' >/dev/null
echo "$out" | jq -e '.[0].packages | test("drivers/postgres")' >/dev/null

# the image ref defaults to GHCR and is stamped once here for downstream reuse
echo "$out" | jq -e '.[0].image == "ghcr.io/sakiladb/postgres:12"' >/dev/null

# full scope: packages collapses to ./...
out=$("$here/build-db-matrix.sh" full '{"mysql":["8","9"]}')
echo "$out" | jq -e 'length == 2' >/dev/null
echo "$out" | jq -e 'all(.packages == "./...")' >/dev/null
echo "$out" | jq -e '[.[].tag] == ["8","9"]' >/dev/null
echo "$out" | jq -e '[.[].image] == ["ghcr.io/sakiladb/mysql:8","ghcr.io/sakiladb/mysql:9"]' >/dev/null

# SAKILADB_REGISTRY overrides the image registry (single source of truth)
out=$(SAKILADB_REGISTRY=example.test/ns "$here/build-db-matrix.sh" narrow '{"postgres":["12"]}')
echo "$out" | jq -e '.[0].image == "example.test/ns/postgres:12"' >/dev/null

# a trailing slash in the override is tolerated (not doubled into an invalid ref)
out=$(SAKILADB_REGISTRY=example.test/ns/ "$here/build-db-matrix.sh" narrow '{"postgres":["12"]}')
echo "$out" | jq -e '.[0].image == "example.test/ns/postgres:12"' >/dev/null

# the DSN must NOT travel through the matrix: GitHub masks the credential and
# drops a job output containing it, which silently empties the matrix.
echo "$out" | jq -e 'all(has("dsn") | not)' >/dev/null

# unknown engine is a hard error, not a silently-empty/null row
if "$here/build-db-matrix.sh" narrow '{"bogus":["1"]}' 2>/dev/null; then
  echo "build-db-matrix_test: FAIL (expected error for unknown engine)" >&2
  exit 1
fi

# invalid scope is rejected
if "$here/build-db-matrix.sh" sideways '{"postgres":["12"]}' 2>/dev/null; then
  echo "build-db-matrix_test: FAIL (expected error for invalid scope)" >&2
  exit 1
fi

echo "build-db-matrix_test: PASS"
