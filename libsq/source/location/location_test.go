package location_test

import (
	"net/url"
	"testing"

	"github.com/neilotoole/sq/libsq/source/location"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
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
			got := location.IsSQLLocation(tc.loc)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestLocationWithPassword(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		loc     string
		pw      string
		want    string
		wantErr bool
	}{
		{
			loc:  "/some/file",
			want: "/some/file",
		},
		{
			loc:  "postgres://sakila:p_ssW0rd@localhost/sakila",
			pw:   "p_ssW0rd",
			want: "postgres://sakila:p_ssW0rd@localhost/sakila",
		},
		{
			loc:  "postgres://sakila:p_ssW0rd@localhost/sakila",
			pw:   `abc_";''\'_*&-  9""'' `,
			want: `postgres://sakila:abc_%22;%27%27%5C%27_%2A&-%20%209%22%22%27%27%20@localhost/sakila`,
		},
		{
			loc:  "postgres://sakila@localhost/sakila",
			pw:   "",
			want: "postgres://sakila@localhost/sakila",
		},
		{
			loc:  "postgres://sakila:p_ssW0rd@localhost/sakila",
			pw:   "",
			want: "postgres://sakila@localhost/sakila",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			t.Parallel()

			beforeURL, err := url.ParseRequestURI(tc.loc)
			require.NoError(t, err)

			got, gotErr := location.LocationWithPassword(tc.loc, tc.pw)
			if tc.wantErr {
				require.Error(t, gotErr)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, got)
			afterURL, err := url.ParseRequestURI(got)
			require.NoError(t, err)

			if tc.pw != "" {
				afterPass, hasPass := afterURL.User.Password()
				require.True(t, hasPass)
				require.Equal(t, tc.pw, afterPass)
			}

			if beforeURL.User != nil {
				require.Equal(t, beforeURL.User.Username(), afterURL.User.Username(),
					"username should not have been modified")
			}
		})
	}
}
