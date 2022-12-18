package source_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
)

func TestRedactedLocation(t *testing.T) {
	testCases := []struct {
		tname string
		loc   string
		want  string
	}{
		{tname: "sqlite", loc: "/path/to/sqlite.db", want: "/path/to/sqlite.db"},
		{tname: "xlsx", loc: "/path/to/data.xlsx", want: "/path/to/data.xlsx"},
		{tname: "https", loc: "https://path/to/data.xlsx", want: "https://path/to/data.xlsx"},
		{tname: "http", loc: "http://path/to/data.xlsx", want: "http://path/to/data.xlsx"},
		{tname: "sqlserver", loc: "sqlserver://sq:p_ssW0rd@localhost?database=sqtest",
			want: "sqlserver://sq:****@localhost?database=sqtest"},
		{tname: "postgres", loc: "postgres://sq:p_ssW0rd@localhost/sqtest?sslmode=disable",
			want: "postgres://sq:****@localhost/sqtest?sslmode=disable"},
		{tname: "mysql", loc: "mysql://sq:p_ssW0rd@localhost:3306/sqtest",
			want: "mysql://sq:****@localhost:3306/sqtest"},
		{tname: "sqlite3", loc: "sqlite3:///path/to/sqlite.db",
			want: "sqlite3:/path/to/sqlite.db"}, // FIXME: how many slashes to we want, or zero slashes?
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.tname, func(t *testing.T) {
			src := &source.Source{Location: tc.loc}
			got := src.RedactedLocation()
			t.Logf("%s  -->  %s", src.Location, got)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestShortLocation(t *testing.T) {
	testCases := []struct {
		tname string
		loc   string
		want  string
	}{
		{tname: "sqlite3_scheme", loc: "sqlite3:///path/to/sqlite.db", want: "sqlite.db"},
		{tname: "sqlite3", loc: "/path/to/sqlite.db", want: "sqlite.db"},
		{tname: "xlsx", loc: "/path/to/data.xlsx", want: "data.xlsx"},
		{tname: "https", loc: "https://path/to/data.xlsx", want: "data.xlsx"},
		{tname: "http", loc: "http://path/to/data.xlsx", want: "data.xlsx"},
		{tname: "sqlserver", loc: "sqlserver://sq:p_ssw0rd@localhost?database=sqtest", want: "sq@localhost/sqtest"},
		{tname: "postgres", loc: "postgres://sq:p_ssW0rd@localhost/sqtest?sslmode=disable",
			want: "sq@localhost/sqtest"},
		{tname: "mysql", loc: "mysql://sq:p_ssW0rd@localhost:3306/sqtest", want: "sq@localhost:3306/sqtest"},
		{tname: "mysql", loc: "mysql://sq:p_ssW0rd@localhost/sqtest", want: "sq@localhost/sqtest"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.tname, func(t *testing.T) {
			got := source.ShortLocation(tc.loc)
			t.Logf("%s  -->  %s", tc.loc, got)
			require.Equal(t, tc.want, got)
		})
	}
}
