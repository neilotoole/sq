package source

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/tu"
)

// Export for testing.
var (
	GroupsFilterOnlyDirectChildren = groupsFilterOnlyDirectChildren
)

func TestParseLoc(t *testing.T) {
	const (
		dbuser = "sakila"
		dbpass = "p_ssW0rd"
	)

	testCases := []struct {
		loc     string
		want    parsedLoc
		wantErr bool
		windows bool
	}{
		{
			loc:  "/path/to/sakila.xlsx",
			want: parsedLoc{name: "sakila", ext: ".xlsx"},
		},
		{
			loc:  "relative/path/to/sakila.xlsx",
			want: parsedLoc{name: "sakila", ext: ".xlsx"},
		},
		{
			loc:  "./relative/path/to/sakila.xlsx",
			want: parsedLoc{name: "sakila", ext: ".xlsx"},
		},
		{
			loc:  "https://server:8080/path/to/sakila.xlsx",
			want: parsedLoc{scheme: "https", hostname: "server", port: 8080, name: "sakila", ext: ".xlsx"},
		},
		{
			loc:  "http://server/path/to/sakila.xlsx?param=val&param2=val2",
			want: parsedLoc{scheme: "http", hostname: "server", name: "sakila", ext: ".xlsx"},
		},
		{
			loc:     "sqlite3:/path/to/sakila.db",
			wantErr: true,
		}, // the scheme is malformed (should be "sqlite3://...")
		{
			loc: "sqlite3:///path/to/sakila.sqlite",
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".sqlite",
				dsn: "/path/to/sakila.sqlite",
			},
		},
		{
			loc:     `sqlite3://C:\path\to\sakila.sqlite`,
			windows: true,
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".sqlite",
				dsn: `C:\path\to\sakila.sqlite`,
			},
		},
		{
			loc:     `sqlite3://C:\path\to\sakila.sqlite?param=val`,
			windows: true,
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".sqlite",
				dsn: `C:\path\to\sakila.sqlite?param=val`,
			},
		},
		{
			loc: "sqlite3:///path/to/sakila",
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", dsn: "/path/to/sakila",
			},
		},
		{
			loc: "sqlite3://path/to/sakila.db",
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".db", dsn: "path/to/sakila.db",
			},
		},
		{
			loc: "sqlite3:///path/to/sakila.db",
			want: parsedLoc{
				typ: typeSL3, scheme: "sqlite3", name: "sakila", ext: ".db", dsn: "/path/to/sakila.db",
			},
		},
		{
			loc: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila",
			want: parsedLoc{
				typ: typeMS, scheme: "sqlserver", user: dbuser, pass: dbpass, hostname: "localhost",
				name: "sakila", dsn: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila",
			},
		},
		{
			loc: "sqlserver://sakila:p_ssW0rd@server:1433?database=sakila",
			want: parsedLoc{
				typ: typeMS, scheme: "sqlserver", user: dbuser, pass: dbpass, hostname: "server",
				port: 1433, name: "sakila",
				dsn: "sqlserver://sakila:p_ssW0rd@server:1433?database=sakila",
			},
		},
		{
			loc: "postgres://sakila:p_ssW0rd@localhost/sakila?sslmode=disable",
			want: parsedLoc{
				typ: typePg, scheme: "postgres", user: dbuser, pass: dbpass, hostname: "localhost",
				name: "sakila", dsn: "dbname=sakila host=localhost password=p_ssW0rd sslmode=disable user=sakila",
			},
		},
		{
			loc: "postgres://sakila:p_ssW0rd@server:5432/sakila?sslmode=disable",
			want: parsedLoc{
				typ: typePg, scheme: "postgres", user: dbuser, pass: dbpass, hostname: "server", port: 5432,
				name: "sakila",
				dsn:  "dbname=sakila host=server password=p_ssW0rd port=5432 sslmode=disable user=sakila",
			},
		},
		{
			loc: "mysql://sakila:p_ssW0rd@localhost/sakila",
			want: parsedLoc{
				typ: typeMy, scheme: "mysql", user: dbuser, pass: dbpass, hostname: "localhost",
				name: "sakila", dsn: "sakila:p_ssW0rd@tcp(localhost:3306)/sakila",
			},
		},
		{
			loc: "mysql://sakila:p_ssW0rd@server:3306/sakila",
			want: parsedLoc{
				typ: typeMy, scheme: "mysql", user: dbuser, pass: dbpass, hostname: "server", port: 3306,
				name: "sakila", dsn: "sakila:p_ssW0rd@tcp(server:3306)/sakila",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tu.Name(1, RedactLocation(tc.loc)), func(t *testing.T) {
			if tc.windows && runtime.GOOS != "windows" {
				return
			}

			tc.want.loc = tc.loc // set this here rather than verbosely in the setup
			got, gotErr := parseLoc(tc.loc)
			if tc.wantErr {
				require.Error(t, gotErr)
				require.Nil(t, got)
				return
			}

			require.NoError(t, gotErr)
			require.Equal(t, tc.want, *got)
		})
	}
}

func TestGroupsFilterOnlyDirectChildren(t *testing.T) {
	testCases := []struct {
		parent string
		groups []string
		want   []string
	}{
		{
			parent: "/",
			groups: []string{"/", "prod", "prod/customer", "staging"},
			want:   []string{"prod", "staging"},
		},
		{
			parent: "prod",
			groups: []string{"/", "prod", "prod/customer", "prod/backup", "staging"},
			want:   []string{"prod/customer", "prod/backup"},
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(tu.Name(i, tc.want), func(t *testing.T) {
			got := GroupsFilterOnlyDirectChildren(tc.parent, tc.groups)
			require.EqualValues(t, tc.want, got)
		})
	}
}
