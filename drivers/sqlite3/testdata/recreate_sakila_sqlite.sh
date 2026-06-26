#!/bin/bash
set -ev

# Requires SQLite >= 3.44 (the film_list / nicer_but_slower_film_list / actor_info
# views use ORDER BY inside group_concat). If your default sqlite3 is older,
# point at a newer one, e.g.:
#   SQLITE3=/opt/homebrew/opt/sqlite/bin/sqlite3 ./recreate_sakila_sqlite.sh
#
# NOTE: a from-scratch rebuild re-stamps every last_update column to the build
# time, because the schema's AFTER INSERT triggers fire during the data load
# (those exact historical timestamps are not in the SQL and can't be derived
# from it). The committed sakila.db preserves the original timestamps; many
# golden tests pin them. So prefer applying schema/view changes incrementally to
# the committed fixture (which keeps the timestamps) rather than committing a
# fresh rebuild, OR update the affected golden test expectations to match.
SQLITE3="${SQLITE3:-sqlite3}"

rm -f sakila.db
"$SQLITE3" sakila.db < ./sqlite-sakila-schema.sql
"$SQLITE3" sakila.db < ./sqlite-sakila-insert-data.sql
"$SQLITE3" sakila.db < ./sqlite-sakila-finalize.sql
