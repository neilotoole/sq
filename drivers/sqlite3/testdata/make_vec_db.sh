#!/bin/bash
# Creates vec.db, a SQLite database with sqlite-vec (vec0) tables.
#
# Usage: ./make_vec_db.sh <sq-binary> <out.db>
#
# The sq binary must have sqlite-vec compiled in (any build of this repo).
set -euo pipefail

if [ $# -ne 2 ]; then
  echo "usage: $0 <sq-binary> <out.db>" >&2
  exit 2
fi

SQ="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
OUT="$2"
CFG="$(mktemp -d)"
trap 'rm -rf "$CFG"' EXIT

rm -f "$OUT"
OUT="$(cd "$(dirname "$OUT")" && pwd)/$(basename "$OUT")"

"$SQ" --config "$CFG" add "sqlite3://$OUT" --handle @vec >/dev/null

run() {
  "$SQ" --config "$CFG" sql --src @vec "$1" >/dev/null
}

run "CREATE TABLE docs(id INTEGER PRIMARY KEY, title TEXT)"
run "INSERT INTO docs(id, title) VALUES (1,'cat'),(2,'dog'),(3,'car')"

run "CREATE VIRTUAL TABLE doc_vec USING vec0(doc_id INTEGER PRIMARY KEY, embedding float[4] distance_metric=cosine, category TEXT, +note TEXT)"
run "INSERT INTO doc_vec(doc_id, embedding, category, note) VALUES (1,'[0.9,0.1,0.0,0.0]','animal','n1')"
run "INSERT INTO doc_vec(doc_id, embedding, category, note) VALUES (2,'[0.8,0.2,0.0,0.0]','animal','n2')"
run "INSERT INTO doc_vec(doc_id, embedding, category, note) VALUES (3,'[0.0,0.0,0.9,0.1]','vehicle','n3')"

run "CREATE VIRTUAL TABLE bin_vec USING vec0(e_i8 int8[4], e_bit bit[8])"
run "INSERT INTO bin_vec(rowid, e_i8, e_bit) VALUES (1, vec_int8('[1,2,3,4]'), vec_bit(X'F0'))"

run "CREATE VIRTUAL TABLE part_vec USING vec0(user_id INTEGER PARTITION KEY, e float[2])"
run "INSERT INTO part_vec(rowid, user_id, e) VALUES (1, 1, '[1.0,2.0]')"
run "INSERT INTO part_vec(rowid, user_id, e) VALUES (2, 2, '[3.0,4.0]')"

run "CREATE VIRTUAL TABLE empty_vec USING vec0(e float[3])"
