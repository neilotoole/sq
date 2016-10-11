#!/usr/bin/env bash
# Smoke tests for sq
# This script should be executed via "make smoke" in the sq root dir



export SQ_LOGFILE="$HOME/.sq/sq.smoke.log"
export SQ_CONFIGFILE="$HOME/.sq/sq.smoke.yml"

cat /dev/null > ${SQ_LOGFILE}
cat /dev/null > ${SQ_CONFIGFILE}

sq add 'mysql://root:root@tcp(localhost:33067)/sq_mydb1' @my1
sq add 'postgres://sq:sq@localhost/pqdb1?sslmode=disable' @pg1
sq add "`pwd`/test/xlsx/test1.xlsx" @excel1
sq add "sqlite3://`pwd`/test/sqlite/sqlite_db1" @sl1

echo ""
sq ls
echo ""
sq ping --all