#!/usr/bin/env bash
# Very simplistic smoke test for sq, until we get comprehensive E2E testing in place.
# This is a basic test of functionality, not a comprehensive suite.
# This script should be executed via "make smoke" in the sq root dir. Data is
# stored in "$HOME/.sq/sq.smoke.log" and "$HOME/.sq/sq.smoke.yml" and those
# files are truncated before each run.
#



export SQ_LOGFILE="$HOME/.sq/sq.smoke.log"
export SQ_CONFIGFILE="$HOME/.sq/sq.smoke.yml"
SQ_TEST_DS_MYSQL=${SQ_TEST_DS_MYSQL:='mysql://sq:sq@tcp(localhost:3306)/sqtest1'}
SQ_TEST_DS_POSTGRES=${SQ_TEST_DS_POSTGRES:='postgres://sq:sq@localhost/sqtest1?sslmode=disable'}

echo ${SQ_TEST_DS_MYSQL}
echo ${SQ_TEST_DS_POSTGRES}

cat /dev/null > ${SQ_LOGFILE}
cat /dev/null > ${SQ_CONFIGFILE}

# Track the return codes for each command. At the end of the test run, if RES > 0,
# then we know there's an error.
RES=$((RES + $?))
function trackResult() {
	R=$?

#	RES=$((RES + R))

	if [ ${R} -ne 0 ]; then
		RES=$((RES + 1))
  	>&2 printf "\n\e[1;97;41m BOLLIX \e[0m\n\n"
  fi

}

sq add ${SQ_TEST_DS_MYSQL} @my1 ; trackResult
sq add ${SQ_TEST_DS_POSTGRES} @pg1 ; trackResult
sq add "sqlite3://`pwd`/test/sqlite/sqlite_db1" @sl1 ; trackResult

sq add "`pwd`/test/xlsx/test_noheader.xlsx" @excel_noheader1 ; trackResult
sq add http://neilotoole.io/sq/test/test1.xlsx @excel_noheader2 ; trackResult
sq add "`pwd`/test/xlsx/test_header.xlsx" @excel_header1 --opts='header=true'; trackResult



sq add "`pwd`/test/csv/user_comma_header.csv" @csv_user_comma_header1 --opts='header=true' ; trackResult
sq add "`pwd`/test/csv/user_comma_noheader.csv" @csv_user_comma_noheader1 ; trackResult
sq add "`pwd`/test/csv/user_pipe_header.csv" @csv_user_pipe_header1 --opts='header=true;delim=pipe' ; trackResult
sq add "`pwd`/test/csv/user_pipe_header.csv" @csv_user_pipe_header2 --opts='header=true;delim=|' ; trackResult
sq add "`pwd`/test/csv/user_semicolon_header.csv" @csv_user_semicolon_header1 --opts='header=true;delim=semi' ; trackResult
sq add "`pwd`/test/csv/user_header.tsv" @tsv_user_header1 --opts='header=true' ; trackResult
sq add "`pwd`/test/csv/user_noheader.tsv" @tsv_user_noheader1 ; trackResult

echo ""
sq ls ; trackResult
echo ""
sq ping --all ; trackResult
echo ""
sq inspect -th @my1 ; trackResult
sq inspect -th @pg1 ; trackResult
sq inspect -th @excel_noheader1 ; trackResult
sq inspect -th @excel_noheader2 ; trackResult
sq inspect -th @excel_header1 ; trackResult
sq inspect -th @sl1 ; trackResult

# inspect is not implemented for csv yet
#sq inspect -th @csv_comma_noheader1 ; trackResult

echo ""

# test some native sql
sq src @my1 ; trackResult
sq -jn 'SELECT * FROM tbluser' | jq -e '. | length == 7' ; trackResult
sq -jn 'SELECT * FROM tbluser' | jq -e '.[0] | length == 3' ; trackResult

sq src @pg1 ; trackResult
sq -jn 'SELECT * FROM tbluser' | jq -e '. | length == 7' ; trackResult
sq -jn 'SELECT * FROM tbluser' | jq -e '.[0] | length == 3' ; trackResult

# A loop might be useful below...
# Check that the returned JSON has 7 rows and 3 cols
sq src  @csv_user_comma_header1 ; trackResult

sq -j '.data' | jq -e '. | length == 7'; trackResult
sq -j '.data' | jq -e '.[0] | length == 3'; trackResult


echo ""
sq src  @csv_user_comma_noheader1 ; trackResult
sq -j '.data' | jq -e '. | length == 7'; trackResult
sq -j '.data' | jq -e '.[0] | length == 3'; trackResult

echo ""
sq src  @csv_user_pipe_header1 ; trackResult
sq -j '.data' | jq -e '. | length == 7'; trackResult
sq -j '.data' | jq -e '.[0] | length == 3'; trackResult

echo ""
sq src  @csv_user_semicolon_header1 ; trackResult
sq -j '.data' | jq -e '. | length == 7'; trackResult
sq -j '.data' | jq -e '.[0] | length == 3'; trackResult

echo ""
sq src  @csv_user_pipe_header1 ; trackResult
sq -j '.data' | jq -e '. | length == 7'; trackResult
sq -j '.data' | jq -e '.[0] | length == 3'; trackResult

echo ""
sq src  @csv_user_pipe_header2 ; trackResult
sq -j '.data' | jq -e '. | length == 7'; trackResult
sq -j '.data' | jq -e '.[0] | length == 3'; trackResult

echo ""
sq src  @tsv_user_header1 ; trackResult
sq -j '.data' | jq -e '. | length == 7'; trackResult
sq -j '.data' | jq -e '.[0] | length == 3'; trackResult

echo ""
sq src @tsv_user_noheader1 ; trackResult
sq -j '.data' | jq -e '. | length == 7'; trackResult
sq -j '.data' | jq -e '.[0] | length == 3'; trackResult



# test some joins

sq -th '@my1.tbluser, @pg1.tbladdress | join(.uid) | .tbluser.uid, .email, .city, .country' ; trackResult
echo ""
sq '@excel_noheader1.user_sheet, @pg1.tbladdress | join(.A == .uid)' ; trackResult
echo ""
sq src @pg1 ; trackResult
sq src ; trackResult
sq src @my1 ; trackResult
sq -th '.tbluser' ; trackResult
sq -th '.tbluser | .[2:4]' ; trackResult
echo ""
sq rm @my1 ; trackResult
sq rm @pg1 ; trackResult
sq rm @excel_noheader1 ; trackResult
sq rm @excel_noheader2 ; trackResult
sq rm @excel_header1 ; trackResult
sq rm @sl1 ; trackResult

echo
echo

if [ ${RES} -eq 0 ]; then
  printf  "\n\e[1;97;42m G'MAN LADS \e[0m\n\n"
  exit 0
else
  >&2 printf "\n\e[1;97;41m FOR FECK'S SAKE LADS \e[0m  ${RES} failed, check log: ${SQ_LOGFILE}\n\n"
  exit 1
fi