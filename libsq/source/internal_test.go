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
		want    ParsedLoc
		wantErr bool
		windows bool
	}{
		{
			loc:  "/path/to/sakila.xlsx",
			want: ParsedLoc{Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:  "relative/path/to/sakila.xlsx",
			want: ParsedLoc{Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:  "./relative/path/to/sakila.xlsx",
			want: ParsedLoc{Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:  "https://server:8080/path/to/sakila.xlsx",
			want: ParsedLoc{Scheme: "https", Hostname: "server", Port: 8080, Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:  "http://server/path/to/sakila.xlsx?param=val&param2=val2",
			want: ParsedLoc{Scheme: "http", Hostname: "server", Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:     "sqlite3:/path/to/sakila.db",
			wantErr: true,
		}, // the scheme is malformed (should be "sqlite3://...")
		{
			loc: "sqlite3:///path/to/sakila.sqlite",
			want: ParsedLoc{
				DriverType: typeSL3, Scheme: "sqlite3", Name: "sakila", Ext: ".sqlite",
				DSN: "/path/to/sakila.sqlite",
			},
		},
		{
			loc:     `sqlite3://C:\path\to\sakila.sqlite`,
			windows: true,
			want: ParsedLoc{
				DriverType: typeSL3, Scheme: "sqlite3", Name: "sakila", Ext: ".sqlite",
				DSN: `C:\path\to\sakila.sqlite`,
			},
		},
		{
			loc:     `sqlite3://C:\path\to\sakila.sqlite?param=val`,
			windows: true,
			want: ParsedLoc{
				DriverType: typeSL3, Scheme: "sqlite3", Name: "sakila", Ext: ".sqlite",
				DSN: `C:\path\to\sakila.sqlite?param=val`,
			},
		},
		{
			loc: "sqlite3:///path/to/sakila",
			want: ParsedLoc{
				DriverType: typeSL3, Scheme: "sqlite3", Name: "sakila", DSN: "/path/to/sakila",
			},
		},
		{
			loc: "sqlite3://path/to/sakila.db",
			want: ParsedLoc{
				DriverType: typeSL3, Scheme: "sqlite3", Name: "sakila", Ext: ".db", DSN: "path/to/sakila.db",
			},
		},
		{
			loc: "sqlite3:///path/to/sakila.db",
			want: ParsedLoc{
				DriverType: typeSL3, Scheme: "sqlite3", Name: "sakila", Ext: ".db", DSN: "/path/to/sakila.db",
			},
		},
		{
			loc: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila",
			want: ParsedLoc{
				DriverType: typeMS, Scheme: "sqlserver", User: dbuser, Pass: dbpass, Hostname: "localhost",
				Name: "sakila", DSN: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila",
			},
		},
		{
			loc: "sqlserver://sakila:p_ssW0rd@server:1433?database=sakila",
			want: ParsedLoc{
				DriverType: typeMS, Scheme: "sqlserver", User: dbuser, Pass: dbpass, Hostname: "server",
				Port: 1433, Name: "sakila",
				DSN: "sqlserver://sakila:p_ssW0rd@server:1433?database=sakila",
			},
		},
		{
			loc: "postgres://sakila:p_ssW0rd@localhost/sakila?sslmode=disable",
			want: ParsedLoc{
				DriverType: typePg, Scheme: "postgres", User: dbuser, Pass: dbpass, Hostname: "localhost",
				Name: "sakila", DSN: "dbname=sakila host=localhost password=p_ssW0rd sslmode=disable user=sakila",
			},
		},
		{
			loc: "postgres://sakila:p_ssW0rd@server:5432/sakila?sslmode=disable",
			want: ParsedLoc{
				DriverType: typePg, Scheme: "postgres", User: dbuser, Pass: dbpass, Hostname: "server", Port: 5432,
				Name: "sakila",
				DSN:  "dbname=sakila host=server password=p_ssW0rd port=5432 sslmode=disable user=sakila",
			},
		},
		{
			loc: "mysql://sakila:p_ssW0rd@localhost/sakila",
			want: ParsedLoc{
				DriverType: typeMy, Scheme: "mysql", User: dbuser, Pass: dbpass, Hostname: "localhost",
				Name: "sakila", DSN: "sakila:p_ssW0rd@tcp(localhost:3306)/sakila",
			},
		},
		{
			loc: "mysql://sakila:p_ssW0rd@server:3306/sakila",
			want: ParsedLoc{
				DriverType: typeMy, Scheme: "mysql", User: dbuser, Pass: dbpass, Hostname: "server", Port: 3306,
				Name: "sakila", DSN: "sakila:p_ssW0rd@tcp(server:3306)/sakila",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tu.Name(1, RedactLocation(tc.loc)), func(t *testing.T) {
			if tc.windows && runtime.GOOS != "windows" {
				return
			}

			tc.want.Loc = tc.loc // set this here rather than verbosely in the setup
			got, gotErr := ParseLocation(tc.loc)
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
