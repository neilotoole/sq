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
			loc:     "duckdb:/path/to/sakila.duckdb",
			wantErr: true,
		}, // the scheme is malformed (should be "duckdb://...")
		{
			loc: "duckdb:///path/to/sakila.duckdb",
			want: Fields{
				DriverType: drivertype.DuckDB, Scheme: "duckdb", Name: "sakila", Ext: ".duckdb",
				DSN: "/path/to/sakila.duckdb",
			},
		},
		{
			loc:     `duckdb://C:\path\to\sakila.duckdb`,
			windows: true,
			want: Fields{
				DriverType: drivertype.DuckDB, Scheme: "duckdb", Name: "sakila", Ext: ".duckdb",
				DSN: `C:\path\to\sakila.duckdb`,
			},
		},
		{
			loc:     `duckdb://C:\path\to\sakila.duckdb?param=val`,
			windows: true,
			want: Fields{
				DriverType: drivertype.DuckDB, Scheme: "duckdb", Name: "sakila", Ext: ".duckdb",
				DSN: `C:\path\to\sakila.duckdb?param=val`,
			},
		},
		{
			loc: "duckdb:///path/to/sakila",
			want: Fields{
				DriverType: drivertype.DuckDB, Scheme: "duckdb", Name: "sakila", DSN: "/path/to/sakila",
			},
		},
		{
			loc: "duckdb://path/to/sakila.db",
			want: Fields{
				DriverType: drivertype.DuckDB, Scheme: "duckdb", Name: "sakila", Ext: ".db", DSN: "path/to/sakila.db",
			},
		},
		{
			loc: "duckdb:///path/to/sakila.db",
			want: Fields{
				DriverType: drivertype.DuckDB, Scheme: "duckdb", Name: "sakila", Ext: ".db", DSN: "/path/to/sakila.db",
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
			loc: "clickhouse://sakila:p_ssW0rd@server:9000/sakila",
			want: Fields{
				DriverType: drivertype.ClickHouse, Scheme: "clickhouse", User: dbuser, Pass: dbpass, Hostname: "server", Port: 9000,
				Name: "sakila",
				DSN:  "clickhouse://sakila:p_ssW0rd@server:9000/sakila",
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

// TestPickSentinels verifies that the placeholder-substitution sentinel
// chooser avoids collisions with literal strings in loc. The sentinel
// format is "9999%03d%07d9999" (4 + salt(3) + idx(7) + 4 = 17 digits);
// if loc legitimately contains such a substring (e.g. a 17-digit number
// in a query parameter), the chooser must bump the salt until no
// candidate collides.
func TestPickSentinels(t *testing.T) {
	tests := []struct {
		name string
		loc  string
		n    int
	}{
		{
			name: "no collision",
			loc:  "postgres://alice:pw@host/db",
			n:    2,
		},
		{
			name: "collision on salt=0",
			// Embed exactly the default-salt sentinel "999900000000009999"
			// in a query parameter; the chooser must move past salt=0.
			loc: "postgres://alice:${env:PW}@host/db?nonce=999900000000009999",
			n:   1,
		},
		{
			name: "collision on multiple consecutive salts",
			// Embed sentinels for salt 0, 1, 2 — the chooser should
			// find salt=3 (or later) and succeed.
			loc: "postgres://alice:${env:PW}@host/db?nonce=999900000000009999&n=999900100000009999&m=999900200000009999",
			n:   1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := pickSentinels(tc.loc, tc.n)
			require.True(t, ok)
			require.Len(t, got, tc.n)
			for _, s := range got {
				require.NotContains(t, tc.loc, s,
					"sentinel %q must not already appear in loc", s)
			}
		})
	}
}

// TestRedact_SentinelCollisionDoesNotMangleQueryParam verifies that
// Redact's restoration step picks the right occurrence of the sentinel
// even when loc legitimately contains the default-salt sentinel string.
// Without the salt-bump in pickSentinels, the literal nonce in the
// query parameter would be rewritten as the placeholder text on
// restoration.
func TestRedact_SentinelCollisionDoesNotMangleQueryParam(t *testing.T) {
	const placeholder = "${env:PW}"
	// 18-digit nonce that, were the chooser to use salt=0, would
	// exactly match the first sentinel.
	const nonce = "999900000000009999"
	loc := "postgres://alice:" + placeholder + "@host/db?nonce=" + nonce
	got := Redact(loc)
	// The query-parameter nonce must survive unchanged — it's a
	// user-provided value, not a sentinel.
	require.Contains(t, got, "nonce="+nonce,
		"literal nonce in query string must not be rewritten by sentinel restoration")
}
