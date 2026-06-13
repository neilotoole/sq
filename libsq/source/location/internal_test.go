package location

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
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

// TestIsRootedNoVolume exercises the predicate behind the Windows
// rooted-no-volume branch of absTemplatePath. The branch itself is
// reachable only on Windows (on Unix a rooted path is absolute, and
// '\' is not a separator), so the backslash and volume-bearing cases
// are GOOS-gated; the separator-free cases hold everywhere.
func TestIsRootedNoVolume(t *testing.T) {
	require.False(t, isRootedNoVolume(""))
	require.False(t, isRootedNoVolume("foo"))
	require.False(t, isRootedNoVolume("./foo"))
	require.True(t, isRootedNoVolume("/foo"),
		"'/' is a separator on every GOOS, and VolumeName('/foo') is empty")

	if runtime.GOOS == "windows" {
		require.True(t, isRootedNoVolume(`\foo`))
		require.False(t, isRootedNoVolume(`C:\foo`), "volume present: absolute, not this case")
		require.False(t, isRootedNoVolume(`C:foo`), "drive-relative: not rooted")
		require.False(t, isRootedNoVolume(`\\server\c$\foo`), "UNC: volume present")
	} else {
		require.False(t, isRootedNoVolume(`\foo`), "'\\' is not a separator on Unix")
	}
}

// TestJoinVolumeTemplatePath verifies the attribution rule of the
// rooted-no-volume join: the volume (filesystem-derived bytes) is
// escaped, while the typed path bytes pass through exactly as typed.
// The table inputs are already-clean backslash-style strings, on which
// filepath.Clean is the identity on every GOOS, so these assertions
// are effective cross-platform even though absTemplatePath reaches
// the join only on Windows. The trailing ".."-cleaning case depends
// on Windows separator semantics and is GOOS-gated.
func TestJoinVolumeTemplatePath(t *testing.T) {
	testCases := []struct {
		volume string
		p      string
		want   string
	}{
		{volume: "", p: `\foo`, want: `\foo`},
		{volume: "C:", p: `\foo\sakila.db`, want: `C:\foo\sakila.db`},
		// UNC administrative share: the volume's '$' is escaped.
		{volume: `\\server\c$`, p: `\data\sakila.db`, want: `\\server\c$$\data\sakila.db`},
		// Typed '$$' in p is preserved as typed, not re-escaped.
		{volume: `\\server\c$`, p: `\q$$x\sakila.db`, want: `\\server\c$$\q$$x\sakila.db`},
	}
	for _, tc := range testCases {
		t.Run(tu.Name(tc.volume, tc.p), func(t *testing.T) {
			require.Equal(t, tc.want, joinVolumeTemplatePath(tc.volume, tc.p))
		})
	}

	// Unescaping the joined template recovers the true filesystem path.
	got := joinVolumeTemplatePath(`\\server\c$`, `\q$$x\sakila.db`)
	require.Equal(t, `\\server\c$\q$x\sakila.db`, secret.Unescape(got))

	if runtime.GOOS == "windows" {
		// Clean resolves ".." segments, matching filepath.Abs semantics.
		require.Equal(t, `C:\bar.db`, joinVolumeTemplatePath("C:", `\foo\..\bar.db`))
	}
}

// TestAbsTemplatePath_RootedNoVolume_Windows exercises the
// rooted-no-volume branch of absTemplatePath end to end: from inside
// a '$'-bearing cwd, a rooted but volume-less path must resolve
// against the cwd's volume root (matching filepath.Abs), not be
// joined under the escaped cwd. Rooted-no-volume paths exist only on
// Windows (on Unix a rooted path is absolute and takes the IsAbs fast
// path), so this test is Windows-only; the helpers it composes are
// covered cross-platform by TestIsRootedNoVolume and
// TestJoinVolumeTemplatePath.
func TestAbsTemplatePath_RootedNoVolume_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("rooted-no-volume paths exist only on Windows")
	}

	dir := filepath.Join(t.TempDir(), "q$exports")
	require.NoError(t, os.Mkdir(dir, 0o750))
	t.Chdir(dir)
	cwd, err := os.Getwd()
	require.NoError(t, err)

	const rooted = `\foo\sakila.db`
	got, err := absTemplatePath(rooted)
	require.NoError(t, err)
	require.Equal(t, secret.Escape(filepath.VolumeName(cwd))+rooted, got)

	// Parity with filepath.Abs: unescaping the template yields exactly
	// what Abs produces for the same input.
	wantAbs, err := filepath.Abs(rooted)
	require.NoError(t, err)
	require.Equal(t, wantAbs, secret.Unescape(got))
}

// TestPickSentinels verifies that the placeholder-substitution sentinel
// chooser avoids collisions with literal strings in loc. The sentinel
// format is "9999%03d%07d9999" (4 + salt(3) + idx(7) + 4 = 18 digits);
// if loc legitimately contains such a substring (e.g. an 18-digit
// number in a query parameter), the chooser must bump the salt until
// no candidate collides.
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

// TestRedact_BestEffortFallbackOnMalformed verifies that even when
// neither secret.ExtractRefs nor the structured DSN parsers can handle
// the input (because of malformed placeholders, broken ports, or
// non-URL-shaped DSNs), Redact still masks credentials best-effort.
// Without this fallback, inputs like "postgres://alice:hunter2@host:bad/db"
// or an ODBC string with "PWD=hunter2" would flow through error
// messages and log lines verbatim.
func TestRedact_BestEffortFallbackOnMalformed(t *testing.T) {
	tests := []struct {
		name      string
		loc       string
		notLeaked string // substring that MUST NOT appear in the redacted form
	}{
		{
			name:      "url with malformed port and unclosed placeholder",
			loc:       "postgres://alice:hunter2@host:invalid_port/db?token=${env:UNCLOSED",
			notLeaked: "hunter2",
		},
		{
			name:      "url with backslash mangling and unclosed placeholder",
			loc:       `postgres://alice:hunter2@host\db?token=${`,
			notLeaked: "hunter2",
		},
		{
			name:      "odbc-style PWD= form",
			loc:       "DRIVER={pg};SERVER=host;UID=alice;PWD=hunter2;TOKEN=${env:UNCLOSED",
			notLeaked: "hunter2",
		},
		{
			name:      "case-insensitive password= form",
			loc:       "Server=host;User=alice;Password=hunter2;Trusted=No;BAD=${",
			notLeaked: "hunter2",
		},
		{
			// Bare "user:pw@host" with no scheme prefix — the regex
			// anchor includes start-of-string so this is masked too.
			name:      "bare userinfo, no scheme",
			loc:       "alice:hunter2@host/db?token=${env:UNCLOSED",
			notLeaked: "hunter2",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Redact(tc.loc)
			require.NotContains(t, got, tc.notLeaked,
				"Redact must mask %q in malformed inputs; got: %s", tc.notLeaked, got)
		})
	}
}
