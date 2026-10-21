#!/bin/bash
# Creates vtab_unopenable.db: a SQLite database holding virtual tables that
# sq cannot open, to test that one unreadable table never breaks a source.
#
# Usage: ./make_vtab_unopenable_db.sh <out.db>
#
# Needs only the stock sqlite3 CLI. The virtual tables are planted by
# writing sqlite_master rows directly, because the modules do not exist.
set -euo pipefail

if [ $# -ne 1 ]; then
  echo "usage: $0 <out.db>" >&2
  exit 2
fi

OUT="$1"
rm -f "$OUT"

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
