package source_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
)

func TestIsSQL(t *testing.T) {
	testCases := []struct {
		loc  string
		want bool
	}{
		{loc: "/path/to/data.xlsx", want: false},
		{loc: "relative/path/to/data.xlsx", want: false},
		{loc: "./relative/path/to/data.xlsx", want: false},
		{loc: "../relative/path/to/data.xlsx", want: false},
		{loc: "https://path/to/data.xlsx", want: false},
		{loc: "http://path/to/data.xlsx", want: false},
		{loc: "sqlite3:///path/to/sqlite.db", want: true},
		{loc: "sqlserver://sq:p_ssW0rd@localhost?database=sqtest", want: true},
		{loc: "postgres://sq:p_ssW0rd@localhost/sqtest?sslmode=disable", want: true},
		{loc: "mysql://sq:p_ssW0rd@tcp(localhost:3306)/sqtest", want: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.loc, func(t *testing.T) {
			got := source.IsSQLLocation(tc.loc)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLocationWithPassword(t *testing.T) {
	const wantPw = `abc_";''\'_*&-  9""'' `

	testCases := []struct {
		loc     string
		pw      string
		want    string
		wantErr bool
	}{
		{
			loc:     "/some/file",
			wantErr: true,
		},
		{
			loc: "postgres://sakila:p_ssW0rd@localhost/sakila",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.loc, func(t *testing.T) {
			beforeURL, err := url.ParseRequestURI(tc.loc)
			require.NoError(t, err)

			got, gotErr := source.LocationWithPassword(tc.loc, tc.pw)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, got)
			afterURL, err := url.ParseRequestURI(got)
			require.NoError(t, err)
			afterPass, ok := afterURL.User.Password()
			require.True(t, ok)
			require.Equal(t, tc.pw, afterPass)

			if beforeURL.User != nil {
				require.Equal(t, beforeURL.User.Username(), afterURL.User.Username(),
					"username should not have been modified")
			}
		})
	}
}
