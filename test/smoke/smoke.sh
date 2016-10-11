#!/usr/bin/env bash
# Smoke tests for sq.
# This is a basic test of functionality, not a comprehensive suite.
# This script should be executed via "make smoke" in the sq root dir



export SQ_LOGFILE="$HOME/.sq/sq.smoke.log"
export SQ_CONFIGFILE="$HOME/.sq/sq.smoke.yml"

cat /dev/null > ${SQ_LOGFILE}
cat /dev/null > ${SQ_CONFIGFILE}

sq add 'mysql://root:root@tcp(localhost:33067)/sq_mydb1' @my1
sq add 'postgres://sq:sq@localhost/pqdb1?sslmode=disable' @pg1
sq add "`pwd`/test/xlsx/test1.xlsx" @excel1
sq add http://neilotoole.io/sq/test/test1.xlsx @excel2
sq add "sqlite3://`pwd`/test/sqlite/sqlite_db1" @sl1
sq add './test/csv/user-comma-noheader.csv' @csv1

echo ""
sq ls
echo ""
sq ping --all
echo ""
sq inspect -th @my1
sq inspect -th @pg1
sq inspect -th @excel1
sq inspect -th @excel2
sq inspect -th @sl1
echo ""
sq -th '@my1.user, @pg1.tbladdress | join(.uid) | .user.uid, .email, .city, .country'
echo ""
sq '@excel1.user_sheet, @pg1.tbladdress | join(.A == .uid)'
echo ""
sq src @pg1
sq src
sq src @my1
sq -th '.user'
sq -th '.user | .[2:4]'
echo ""
sq rm @my1
sq rm @pg1
sq rm @excel1
sq rm @excel2
sq rm @sl1

