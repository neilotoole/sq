#!/usr/bin/env bash
# Very simplistic smoke test for sq, until we get comprehensive E2E testing in place.
# This is a basic test of functionality, not a comprehensive suite.
# This script should be executed via "make smoke" in the sq root dir. Data is
# stored in "$HOME/.sq/sq.smoke.log" and "$HOME/.sq/sq.smoke.yml" and those
# files are truncated before each run.
#



export SQ_LOGFILE="$HOME/.sq/sq.smoke.log"
export SQ_CONFIGFILE="$HOME/.sq/sq.smoke.yml"

cat /dev/null > ${SQ_LOGFILE}
cat /dev/null > ${SQ_CONFIGFILE}

# Track the return codes for each command. At the end of the test run, if RES > 0,
# then we know there's an error.
RES=$((RES + $?))
function trackResult() {
	R=$?
	RES=$((RES + R))

	if [ ${R} -ne 0 ]; then
  	>&2 printf "\n\e[1;97;41m BOLLIX \e[0m\n\n"
  fi

}

sq add 'mysql://root:root@tcp(localhost:33067)/sq_mydb1' @my1 ; trackResult

sq add 'postgres://sq:sq@localhost/pqdb1?sslmode=disable' @pg1 ; trackResult
sq add "`pwd`/test/xlsx/test1.xlsx" @excel1 ; trackResult
sq add http://neilotoole.io/sq/test/test1.xlsx @excel2 ; trackResult
sq add "sqlite3://`pwd`/test/sqlite/sqlite_db1" @sl1 ; trackResult

sq add "`pwd`/test/csv/user_comma_header.csv" @csv_user_comma_header1 ; trackResult
sq add "`pwd`/test/csv/user_comma_noheader.csv" @csv_user_comma_noheader1 ; trackResult
sq add "`pwd`/test/csv/user_pipe_header.csv" @csv_user_pipe_header1 ; trackResult
sq add "`pwd`/test/csv/user_semicolon_header.csv" @csv_user_semicolon_header1 ; trackResult
sq add "`pwd`/test/csv/user_header.tsv" @tsv_user_header1 ; trackResult
sq add "`pwd`/test/csv/user_noheader.tsv" @tsv_user_noheader1 ; trackResult

echo ""
sq ls ; trackResult
echo ""
sq ping --all ; trackResult
echo ""
sq inspect -th @my1 ; trackResult
sq inspect -th @pg1 ; trackResult
sq inspect -th @excel1 ; trackResult
sq inspect -th @excel2 ; trackResult
sq inspect -th @sl1 ; trackResult
#sq inspect -th @csv_comma_noheader1 ; trackResult

echo ""
#sq src @csv_user_comma_header1 ; trackResult
sq -th '@csv_user_comma_header1.data' ; trackResult
# Check that the returned JSON has three columns...
sq -j '@csv_user_comma_header1.data' | jq -e '.[0] | length == 3'; trackResult
echo ""
sq -th '@csv_user_comma_noheader1.data' ; trackResult
sq -j '@csv_user_comma_noheader1.data' | jq -e '.[0] | length == 3'; trackResult
echo ""
sq -th '@csv_user_pipe_header1.data' ; trackResult
sq -j '@csv_user_pipe_header1.data' | jq -e '.[0] | length == 3'; trackResult
echo ""
sq -th '@csv_user_semicolon_header1.data' ; trackResult
sq -j '@csv_user_semicolon_header1.data' | jq -e '.[0] | length == 3'; trackResult
echo ""
sq -th '@csv_user_pipe_header1.data' ; trackResult
sq -j '@csv_user_pipe_header1.data' | jq -e '.[0] | length == 3'; trackResult
echo ""
sq -th '@tsv_user_header1.data' ; trackResult
sq -j '@tsv_user_header1.data' | jq -e '.[0] | length == 3'; trackResult
echo ""



sq -th '@my1.user, @pg1.tbladdress | join(.uid) | .user.uid, .email, .city, .country' ; trackResult
echo ""
sq '@excel1.user_sheet, @pg1.tbladdress | join(.A == .uid)' ; trackResult
echo ""
sq src @pg1 ; trackResult
sq src ; trackResult
sq src @my1 ; trackResult
sq -th '.user' ; trackResult
sq -th '.user | .[2:4]' ; trackResult
echo ""
sq rm @my1 ; trackResult
sq rm @pg1 ; trackResult
sq rm @excel1 ; trackResult
sq rm @excel2 ; trackResult
sq rm @sl1 ; trackResult

echo
echo

if [ ${RES} -eq 0 ]; then
  printf  "\n\e[1;97;42m G'MAN LADS \e[0m\n\n"
  exit 0
else
  >&2 printf "\n\e[1;97;41m FOR FECK'S SAKE LADS \e[0m  smoke test failed, check log: ${SQ_LOGFILE}\n\n"
  exit 1
fi