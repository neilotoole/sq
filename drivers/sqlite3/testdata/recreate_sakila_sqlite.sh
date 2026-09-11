#!/bin/bash
set -ev

# Requires SQLite >= 3.44 (the film_list / nicer_but_slower_film_list / actor_info
# views use ORDER BY inside group_concat). If your default sqlite3 is older,
# point at a newer one, e.g.:
#   SQLITE3=/opt/homebrew/opt/sqlite/bin/sqlite3 ./recreate_sakila_sqlite.sh
#
# The insert-data file carries the canonical original last_update timestamps
# (2006-02-15, etc.). The schema's per-table AFTER INSERT/UPDATE triggers set
# last_update to DATETIME('NOW'), so they are dropped before the data load and
# recreated afterwards. Without this, a rebuild would clobber every timestamp
# with the build date.
SQLITE3="${SQLITE3:-sqlite3}"
TRIGGERS="${TMPDIR:-/tmp}/sqlite_sakila_triggers.$$.sql"

rm -f sakila.db
"$SQLITE3" sakila.db < ./sqlite-sakila-schema.sql
# Save the trigger definitions, then drop them so the data load preserves the
# file's timestamps.
"$SQLITE3" sakila.db "SELECT sql || ';' FROM sqlite_master WHERE type='trigger';" > "$TRIGGERS"
"$SQLITE3" sakila.db "SELECT 'DROP TRIGGER ' || name || ';' FROM sqlite_master WHERE type='trigger';" | "$SQLITE3" sakila.db
"$SQLITE3" sakila.db < ./sqlite-sakila-insert-data.sql
"$SQLITE3" sakila.db < ./sqlite-sakila-finalize.sql
# Recreate the triggers on the loaded data (they do not fire retroactively).
"$SQLITE3" sakila.db < "$TRIGGERS"
rm -f "$TRIGGERS"
