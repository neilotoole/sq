#!/usr/bin/env bash

set -e
# This script shows how to generate CSV/TSV files for each table in a data source.

# Source: @sakila_sqlite3 from ${SQ_ROOT}/drivers/sqlite3/testdata/sakila.db
# CSV files are output into ./sakila-csv and ./sakila-csv-noheader
# TSV files are output into ./sakila-tsv and ./sakila-tsv-noheader

mkdir -p sakila-csv
sq inspect @sakila_sqlite3 -j | jq -r '.tables[] | .name' | xargs -I % sq @sakila_sqlite3.% --csv --output ./sakila-csv/%.csv
mkdir -p sakila-csv-noheader
sq inspect @sakila_sqlite3 -j | jq -r '.tables[] | .name' | xargs -I % sq @sakila_sqlite3.% --csv --no-header --output ./sakila-csv-noheader/%.csv

mkdir -p sakila-tsv
sq inspect @sakila_sqlite3 -j | jq -r '.tables[] | .name' | xargs -I % sq @sakila_sqlite3.% --tsv --output ./sakila-tsv/%.tsv
mkdir -p sakila-tsv-noheader
sq inspect @sakila_sqlite3 -j | jq -r '.tables[] | .name' | xargs -I % sq @sakila_sqlite3.% --tsv --no-header --output ./sakila-tsv-noheader/%.tsv
