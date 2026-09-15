#!/usr/bin/env bash
# Regenerates drivers/parquet/testdata/sakila-parquet/ by converting
# drivers/csv/testdata/sakila-csv/ to Parquet via DuckDB.
#
# Requires: duckdb (https://duckdb.org/docs/installation/).
set -euo pipefail

cd "$(dirname "$0")"
SRC_DIR="../../csv/testdata/sakila-csv"
OUT_DIR="sakila-parquet"
mkdir -p "$OUT_DIR"

for csv in "$SRC_DIR"/*.csv; do
    name=$(basename "$csv" .csv)
    out="$OUT_DIR/$name.parquet"
    echo "Converting $csv -> $out"
    duckdb :memory: <<SQL
COPY (SELECT * FROM read_csv_auto('$csv', header=true)) TO '$out' (FORMAT PARQUET);
SQL
done

echo "Generated sakila parquet fixtures in $OUT_DIR/"
ls -la -- "$OUT_DIR/"*.parquet
