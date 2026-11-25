#!/usr/bin/env bash
# generate.sh regenerates the parquet fixture files in this directory using
# the DuckDB CLI. Re-run after editing the schema/values below.
#
# Requires: duckdb (https://duckdb.org/docs/installation/).
set -euo pipefail

cd "$(dirname "$0")"

duckdb :memory: <<'SQL'
COPY (
  SELECT 1 AS actor_id, 'PENELOPE' AS first_name, 'GUINESS' AS last_name,
         TIMESTAMP '2006-02-15 04:34:33' AS last_update
  UNION ALL SELECT 2, 'NICK', 'WAHLBERG', TIMESTAMP '2006-02-15 04:34:33'
  UNION ALL SELECT 3, 'ED', 'CHASE', TIMESTAMP '2006-02-15 04:34:33'
) TO 'actor.parquet' (FORMAT PARQUET);

COPY (
  SELECT 1 AS id,
         {'name': 'Alice', 'age': 30} AS person,
         [1, 2, 3] AS scores,
         MAP {'a': 1, 'b': 2} AS counts
) TO 'nested.parquet' (FORMAT PARQUET);

COPY (
  SELECT CAST(1.23 AS DECIMAL(5,2)) AS small,
         CAST(1234567.890123 AS DECIMAL(18,6)) AS medium,
         CAST(12345678901234567890.123456789012345678 AS DECIMAL(38,18)) AS big
) TO 'decimals.parquet' (FORMAT PARQUET);

COPY (
  SELECT TIMESTAMP '2026-06-06 12:34:56' AS ts_us,
         CAST(TIMESTAMP '2026-06-06 12:34:56' AS TIMESTAMP_MS) AS ts_ms,
         CAST(TIMESTAMP '2026-06-06 12:34:56' AS TIMESTAMP_NS) AS ts_ns,
         CAST(TIMESTAMP '2026-06-06 12:34:56' AS TIMESTAMPTZ) AS ts_tz
) TO 'timestamps.parquet' (FORMAT PARQUET);

COPY (
  SELECT 1 AS id, 'one' AS name, 1.5 AS score
  UNION ALL SELECT NULL, NULL, NULL
  UNION ALL SELECT 3, 'three', NULL
) TO 'nulls.parquet' (FORMAT PARQUET);

COPY (
  SELECT 1 AS id, 'x' AS name WHERE FALSE
) TO 'empty.parquet' (FORMAT PARQUET);
SQL

# truncated.parquet: cut the last 16 bytes off actor.parquet so the footer
# magic is gone but the head magic remains.
SIZE=$(wc -c < actor.parquet)
TRUNC=$((SIZE - 16))
head -c "$TRUNC" actor.parquet > truncated.parquet

echo "Generated fixtures in $(pwd)"
ls -la -- *.parquet
