#!/bin/bash
# Creates vtab_unopenable.db: a SQLite database holding virtual tables that
# sq cannot open, to test that one unreadable table never breaks a source.
#
# Usage: ./make_vtab_unopenable_db.sh <out.db> [only]
#
# With the second arg "only", the database holds just fx_other (no ordinary
# tables), so every table in the source is unopenable.
#
# Needs only the stock sqlite3 CLI. The virtual tables are planted by
# writing sqlite_master rows directly, because the modules do not exist.
set -euo pipefail

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
  echo "usage: $0 <out.db> [only]" >&2
  exit 2
fi

OUT="$1"
rm -f "$OUT"

if [ "${2:-}" = "only" ]; then
  # sqlite3 creates the file lazily; give it a header.
  sqlite3 "$OUT" 'PRAGMA user_version=1;'
  sqlite3 "$OUT" <<'SQL'
PRAGMA writable_schema=ON;
INSERT INTO sqlite_master(type, name, tbl_name, rootpage, sql) VALUES
 ('table', 'fx_other', 'fx_other', 0,
  'CREATE VIRTUAL TABLE fx_other USING vectorlite(my_embedding float32[3], hnsw(max_elements=100))');
PRAGMA writable_schema=OFF;
SQL
  exit 0
fi

sqlite3 "$OUT" <<'SQL'
CREATE TABLE plain(id INTEGER PRIMARY KEY);
INSERT INTO plain VALUES (1), (2);
PRAGMA writable_schema=ON;
INSERT INTO sqlite_master(type, name, tbl_name, rootpage, sql) VALUES
 ('table', 'fx_rescore', 'fx_rescore', 0,
  'CREATE VIRTUAL TABLE fx_rescore USING vec0(embedding FLOAT[384] distance_metric=cosine INDEXED BY rescore(quantizer=bit, oversample=50))'),
 ('table', 'fx_other', 'fx_other', 0,
  'CREATE VIRTUAL TABLE fx_other USING vectorlite(my_embedding float32[3], hnsw(max_elements=100))');
PRAGMA writable_schema=OFF;
SQL
