package location

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh/tu"
)

func TestParse(t *testing.T) {
	const (
		dbuser = "sakila"
		dbpass = "p_ssW0rd"
	)

	testCases := []struct {
		loc     string
		want    Fields
		wantErr bool
		windows bool
	}{
		{
			loc:  "/path/to/sakila.xlsx",
			want: Fields{Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:  "relative/path/to/sakila.xlsx",
			want: Fields{Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:  "./relative/path/to/sakila.xlsx",
			want: Fields{Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:  "https://server:8080/path/to/sakila.xlsx",
			want: Fields{Scheme: "https", Hostname: "server", Port: 8080, Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:  "http://server/path/to/sakila.xlsx?param=val&param2=val2",
			want: Fields{Scheme: "http", Hostname: "server", Name: "sakila", Ext: ".xlsx"},
		},
		{
			loc:     "sqlite3:/path/to/sakila.db",
			wantErr: true,
		}, // the scheme is malformed (should be "sqlite3://...")
		{
			loc: "sqlite3:///path/to/sakila.sqlite",
			want: Fields{
				DriverType: drivertype.SQLite, Scheme: "sqlite3", Name: "sakila", Ext: ".sqlite",
				DSN: "/path/to/sakila.sqlite",
			},
		},
		{
			loc:     `sqlite3://C:\path\to\sakila.sqlite`,
			windows: true,
			want: Fields{
				DriverType: drivertype.SQLite, Scheme: "sqlite3", Name: "sakila", Ext: ".sqlite",
				DSN: `C:\path\to\sakila.sqlite`,
			},
		},
		{
			loc:     `sqlite3://C:\path\to\sakila.sqlite?param=val`,
			windows: true,
			want: Fields{
				DriverType: drivertype.SQLite, Scheme: "sqlite3", Name: "sakila", Ext: ".sqlite",
				DSN: `C:\path\to\sakila.sqlite?param=val`,
			},
		},
		{
			loc: "sqlite3:///path/to/sakila",
			want: Fields{
				DriverType: drivertype.SQLite, Scheme: "sqlite3", Name: "sakila", DSN: "/path/to/sakila",
			},
		},
		{
			loc: "sqlite3://path/to/sakila.db",
			want: Fields{
				DriverType: drivertype.SQLite, Scheme: "sqlite3", Name: "sakila", Ext: ".db", DSN: "path/to/sakila.db",
			},
		},
		{
			loc: "sqlite3:///path/to/sakila.db",
			want: Fields{
				DriverType: drivertype.SQLite, Scheme: "sqlite3", Name: "sakila", Ext: ".db", DSN: "/path/to/sakila.db",
			},
		},
		{
			loc: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila",
			want: Fields{
				DriverType: drivertype.MSSQL, Scheme: "sqlserver", User: dbuser, Pass: dbpass, Hostname: "localhost",
				Name: "sakila", DSN: "sqlserver://sakila:p_ssW0rd@localhost?database=sakila",
			},
		},
		{
			loc: "sqlserver://sakila:p_ssW0rd@server:1433?database=sakila",
			want: Fields{
				DriverType: drivertype.MSSQL, Scheme: "sqlserver", User: dbuser, Pass: dbpass, Hostname: "server",
				Port: 1433, Name: "sakila",
				DSN: "sqlserver://sakila:p_ssW0rd@server:1433?database=sakila",
			},
		},
		{
			loc: "postgres://sakila:p_ssW0rd@localhost/sakila?sslmode=disable",
			want: Fields{
				DriverType: drivertype.Pg, Scheme: "postgres", User: dbuser, Pass: dbpass, Hostname: "localhost",
				Name: "sakila", DSN: "dbname=sakila host=localhost password=p_ssW0rd sslmode=disable user=sakila",
			},
		},
		{
			loc: "postgres://sakila:p_ssW0rd@server:5432/sakila?sslmode=disable",
			want: Fields{
				DriverType: drivertype.Pg, Scheme: "postgres", User: dbuser, Pass: dbpass, Hostname: "server", Port: 5432,
				Name: "sakila",
				DSN:  "dbname=sakila host=server password=p_ssW0rd port=5432 sslmode=disable user=sakila",
			},
		},
		{
			loc: "mysql://sakila:p_ssW0rd@localhost/sakila",
			want: Fields{
				DriverType: drivertype.MySQL, Scheme: "mysql", User: dbuser, Pass: dbpass, Hostname: "localhost",
				Name: "sakila", DSN: "sakila:p_ssW0rd@tcp(localhost:3306)/sakila",
			},
		},
		{
			loc: "mysql://sakila:p_ssW0rd@server:3306/sakila",
			want: Fields{
				DriverType: drivertype.MySQL, Scheme: "mysql", User: dbuser, Pass: dbpass, Hostname: "server", Port: 3306,
				Name: "sakila", DSN: "sakila:p_ssW0rd@tcp(server:3306)/sakila",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(1, Redact(tc.loc)), func(t *testing.T) {
			if tc.windows && runtime.GOOS != "windows" {
				return
			}

			tc.want.Loc = tc.loc // set this here rather than verbosely in the setup
			got, gotErr := Parse(tc.loc)
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
