package source_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/drivers/csv"
	"github.com/neilotoole/sq/drivers/mysql"
	"github.com/neilotoole/sq/drivers/postgres"
	"github.com/neilotoole/sq/drivers/sqlite3"
	"github.com/neilotoole/sq/drivers/sqlserver"
	"github.com/neilotoole/sq/drivers/xlsx"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
)

func TestVerifyLegalHandle(t *testing.T) {
	fails := []struct {
		handle string
		msg    string
	}{
		{"", "empty is invalid"},
		{"  ", "no whitespace"},
		{"handle", "must start with @"},
		{"@", "needs at least one char"},
		{"1handle", "must start with @"},
		{"@ handle", "no whitespace"},
		{"@handle ", "no whitespace"},
		{"@handle#", "no special chars"},
		{"@1handle", "2nd char must be letter"},
		{"@1", "2nd char must be letter"},
		{"@?handle", "2nd char must be letter"},
		{"@?handle#", "no special chars"},
		{"@ha\nndle", "no newlines"},
	}

	for i, fail := range fails {
		require.Error(t, source.VerifyLegalHandle(fail.handle), fmt.Sprintf("[%d] %s]", i, fail.msg))
	}

	passes := []string{
		"@handle",
		"@handle1",
		"@h1",
		"@h_",
		"@h__",
		"@h__1",
		"@h__1__a___",
	}

	for i, pass := range passes {
		require.Nil(t, source.VerifyLegalHandle(pass), fmt.Sprintf("[%d] should pass", i))
	}
}

func TestSuggestHandle(t *testing.T) {
	testCases := []struct {
		typ   source.Type
		loc   string
		want  string
		taken []string
	}{
		{typ: csv.TypeCSV, loc: "/path/to/actor.csv", want: "@actor_csv"},
		{typ: source.TypeNone, loc: "/path/to/actor.csv", want: "@actor_csv"},
		{typ: xlsx.Type, loc: "/path/to/sakila.xlsx", want: "@sakila_xlsx"},
		{typ: xlsx.Type, loc: "/path/to/123_sakila.xlsx", want: "@h123_sakila_xlsx"},
		{typ: xlsx.Type, loc: "/path/to/__sakila.xlsx", want: "@h__sakila_xlsx"},
		{typ: xlsx.Type, loc: "/path/to/sakila.something.xlsx", want: "@sakila_something_xlsx"},
		{typ: xlsx.Type, loc: "/path/to/😀abc123😀", want: "@h_abc123__xlsx"},
		{typ: source.TypeNone, loc: "/path/to/sakila.xlsx", want: "@sakila_xlsx"},
		{
			typ: xlsx.Type, loc: "/path/to/sakila.xlsx", want: "@sakila_xlsx_2",
			taken: []string{"@sakila_xlsx", "@sakila_xlsx_1"},
		},
		{typ: sqlite3.Type, loc: "sqlite3:///path/to/sakila.db", want: "@sakila_sqlite"},
		{typ: source.TypeNone, loc: "sqlite3:///path/to/sakila.db", want: "@sakila_sqlite"},
		{typ: sqlite3.Type, loc: "/path/to/sakila.db", want: "@sakila_sqlite"},
		{typ: sqlserver.Type, loc: "sqlserver://sakila_p_ssW0rd@localhost?database=sakila", want: "@sakila_mssql"},
		{typ: source.TypeNone, loc: "sqlserver://sakila_p_ssW0rd@localhost?database=sakila", want: "@sakila_mssql"},
		{
			typ: source.TypeNone, loc: "sqlserver://sakila_p_ssW0rd@localhost?database=sakila", want: "@sakila_mssql_1",
			taken: []string{"@sakila_mssql"},
		},
		{typ: postgres.Type, loc: "postgres://sakila_p_ssW0rd@localhost/sakila?sslmode=disable", want: "@sakila_pg"},
		{typ: source.TypeNone, loc: "postgres://sakila_p_ssW0rd@localhost/sakila?sslmode=disable", want: "@sakila_pg"},
		{typ: postgres.Type, loc: "postgres://sakila_p_ssW0rd@localhost/sakila?sslmode=disable", want: "@sakila_pg"},
		{typ: mysql.Type, loc: "mysql://sakila_p_ssW0rd@localhost:3306/sakila", want: "@sakila_my"},
		{typ: source.TypeNone, loc: "mysql://sakila_p_ssW0rd@localhost:3306/sakila", want: "@sakila_my"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.typ.String()+"__"+tc.loc, func(t *testing.T) {
			takenFn := func(handle string) bool {
				return stringz.InSlice(tc.taken, handle)
			}

			got, err := source.SuggestHandle(tc.typ, tc.loc, takenFn)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseTableHandle(t *testing.T) {
	testCases := []struct {
		input  string
		valid  bool
		handle string
		table  string
	}{
		{"@handle1", true, "@handle1", ""},
		{"  @handle1 ", true, "@handle1", ""},
		{"@handle1.tbl1", true, "@handle1", "tbl1"},
		{"  @handle1.tbl1  ", true, "@handle1", "tbl1"},
		{"@handle1 .tbl1", false, "", ""},
		{"@handle1. tbl1", false, "", ""},
		{"@handle1 . tbl1", false, "", ""},
		{".tbl1", true, "", "tbl1"},
		{" .tbl1 ", true, "", "tbl1"},
		{" ._tbl1 ", true, "", "_tbl1"},
		{"invalidhandle", false, "", ""},
		{"invalidhandle.tbl1", false, "", ""},
		{"invalidhandle.@tbl1", false, "", ""},
		{".invalid table", false, "", ""},
		{"", false, "", ""},
		{"  ", false, "", ""},
	}

	for i, tc := range testCases {
		tc := tc

		t.Run(fmt.Sprintf("[%d]__%s", i, tc.input), func(t *testing.T) {
			handle, table, err := source.ParseTableHandle(tc.input)
			if tc.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
			assert.Equal(t, tc.handle, handle)
			assert.Equal(t, tc.table, table)
		})
	}
}
