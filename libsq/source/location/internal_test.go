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
			// sqlite3 with a "?param" suffix: the params are snipped off
			// the name/ext but retained in the DSN.
			loc: "sqlite3:///path/to/sakila.db?_auth_pass=hunter2",
			want: Fields{
				DriverType: drivertype.SQLite, Scheme: "sqlite3", Name: "sakila", Ext: ".db",
				DSN: "/path/to/sakila.db?_auth_pass=hunter2",
			},
		},
		{
			// duckdb with a "?param" suffix.
			loc: "duckdb:///path/to/sakila.duckdb?access_mode=READ_ONLY",
			want: Fields{
				DriverType: drivertype.DuckDB, Scheme: "duckdb", Name: "sakila", Ext: ".duckdb",
				DSN: "/path/to/sakila.duckdb?access_mode=READ_ONLY",
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
		{
			loc: "oracle://sakila:p_ssW0rd@server:1521/sakila",
			want: Fields{
				DriverType: drivertype.Oracle, Scheme: "oracle", User: dbuser, Pass: dbpass, Hostname: "server", Port: 1521,
				Name: "sakila", DSN: "oracle://sakila:p_ssW0rd@server:1521/sakila",
			},
		},
		{
			// ClickHouse database via the path component.
			loc: "clickhouse://sakila:p_ssW0rd@localhost/sakila",
			want: Fields{
				DriverType: drivertype.ClickHouse, Scheme: "clickhouse", User: dbuser, Pass: dbpass, Hostname: "localhost",
				Name: "sakila", DSN: "clickhouse://sakila:p_ssW0rd@localhost:9000/sakila",
			},
		},
		{
			// ClickHouse database via the ?database= query param (empty path).
			loc: "clickhouse://sakila:p_ssW0rd@localhost:9000?database=mydb",
			want: Fields{
				DriverType: drivertype.ClickHouse, Scheme: "clickhouse", User: dbuser, Pass: dbpass, Hostname: "localhost",
				Port: 9000, Name: "mydb", DSN: "clickhouse://sakila:p_ssW0rd@localhost:9000/?database=mydb",
			},
		},
		{
			// rqlite: full userinfo, port, and path.
			loc: "rqlite://sakila:p_ssW0rd@server:4001/sakila",
			want: Fields{
				DriverType: drivertype.Rqlite, Scheme: "rqlite", User: dbuser, Pass: dbpass, Hostname: "server",
				Port: 4001, Name: "sakila", DSN: "rqlite://sakila:p_ssW0rd@server:4001/sakila",
			},
		},
		{
			// rqlite without userinfo.
			loc: "rqlite://server:4001",
			want: Fields{
				DriverType: drivertype.Rqlite, Scheme: "rqlite", Hostname: "server", Port: 4001,
				DSN: "rqlite://server:4001",
			},
		},
		{
			// rqlite with a bad (non-numeric) port: error.
			loc:     "rqlite://server:notaport",
			wantErr: true,
		},
		{
			// rqlite malformed (unterminated IPv6 bracket): error.
			loc:     "rqlite://server:4001/db?x=y\x7f",
			wantErr: true,
		},
		{
			// sqlserver with a bad port: error.
			loc:     "sqlserver://sakila:p_ssW0rd@server:notaport?database=sakila",
			wantErr: true,
		},
		{
			// HTTP URL with a non-numeric port: Atoi fails, error.
			loc:     "http://server:notaport/data.csv",
			wantErr: true,
		},
		{
			// A scheme dburl accepts but sq does not support hits the
			// switch default and errors (without echoing the password).
			loc:     "cockroachdb://sakila:p_ssW0rd@server/sakila",
			wantErr: true,
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

// TestRedactRaw_EdgeCases exercises redactRaw branches not reached via
// the well-formed inputs in the external Redact tests: the empty-string
// fast path and the http parse-error fallback to best-effort redaction.
func TestRedactRaw_EdgeCases(t *testing.T) {
	require.Equal(t, "", redactRaw("", nil))

	// A malformed http URL that url.ParseRequestURI rejects (control
	// char) must fall back to best-effort redaction, not leak the
	// inline password.
	got := redactRaw("http://alice:hunter2@host/db\x7f", nil)
	require.NotContains(t, got, "hunter2")
	require.Contains(t, got, "xxxxx")

	// Same for a malformed rqlite URL.
	got = redactRaw("rqlite://alice:hunter2@[::1", nil)
	require.NotContains(t, got, "hunter2")
	require.Contains(t, got, "xxxxx")

	// A DSN that dburl cannot parse falls back to best-effort too.
	got = redactRaw("postgres://alice:hunter2@host:badport/db", nil)
	require.NotContains(t, got, "hunter2")
	require.Contains(t, got, "xxxxx")
}

// TestReplaceSecretQueryValues_UnescapeFallback covers the branch where
// a query key cannot be URL-unescaped (a stray '%' not followed by two
// hex digits): the raw key is used for the secret check instead, and a
// secret-bearing key is still masked.
func TestReplaceSecretQueryValues_UnescapeFallback(t *testing.T) {
	// "%zz" is not a valid percent-escape, so QueryUnescape fails and
	// the raw key is used for the secret check. The raw key still ends
	// in "password", so the value is masked.
	got := replaceSecretQueryValues("my%zzpassword=hunter2", "xxxxx", nil)
	require.NotContains(t, got, "hunter2")
	require.Contains(t, got, "xxxxx")

	// A non-secret key with an invalid escape: unescape fails, raw key
	// used, no match, value left intact.
	got = replaceSecretQueryValues("my%zzkey=plainval", "xxxxx", nil)
	require.Equal(t, "my%zzkey=plainval", got)
}

// TestRedact_NoRefsWithDollar covers the Redact branch where loc
// contains "${" but no well-formed placeholder ref, which the
// table-driven external test doesn't hit.
func TestRedact_NoRefsWithDollar(t *testing.T) {
	// loc contains "${" but ExtractRefs finds no well-formed ref (the
	// brace is unterminated), so Redact takes the redactRaw(loc, nil)
	// path. The inline password must still be masked.
	got := Redact("postgres://alice:hunter2@host/db?x=${")
	require.NotContains(t, got, "hunter2")
}

// TestIsFpath exercises the heuristics in isFpath that decide whether a
// location is a plain filesystem path (vs a URL, a driver-scheme DSN, or
// a placeholder template).
func TestIsFpath(t *testing.T) {
	testCases := []struct {
		loc    string
		wantOK bool
	}{
		{loc: "/abs/path/data.csv", wantOK: true},
		{loc: "relative/data.csv", wantOK: true},
		{loc: "data.csv", wantOK: true},
		// '$$' escapes to a literal '$': ref-free, still a path.
		{loc: "q$$exports/data.csv", wantOK: true},
		// A colon is legal in a filename on Unix: the leading token isn't a
		// known scheme, so these stay paths rather than being read as a
		// driver DSN (gh #859).
		{loc: "./dump.sqlite3:old.db", wantOK: true},
		{loc: "./my-duckdb:thing.db", wantOK: true},
		// A bare Windows volume "C:" is not a known scheme, so a drive path
		// is treated as a path, dissolving the gh #797 trap without a
		// dedicated drive-letter branch.
		{loc: `C:\db.sqlite`, wantOK: true},
		// An unknown scheme with no "://" authority is treated as a path,
		// consistent with the no-slash form already being a path.
		{loc: "weird:notes.db", wantOK: true},
		{loc: "weird:/notes/x.db", wantOK: true},
		// A leading token matching a *network* DB scheme but lacking a
		// "://" authority is a colon-bearing filename, not a DSN (network
		// DSNs are always "scheme://..."), so it stays a path. Only the
		// file-DB schemes have a bare "scheme:path" DSN form.
		{loc: "postgres:notes.csv", wantOK: true},
		{loc: "mysql:data.csv", wantOK: true},
		// Any "scheme://" authority form is a URL, never a path, whether
		// the scheme is known or not (the latter avoids mangling a mistyped
		// URL into a garbage path before it fails downstream).
		{loc: "http://acme.com/data.csv", wantOK: false},
		{loc: "https://acme.com/data.csv", wantOK: false},
		{loc: "postgres://u:p@host/db", wantOK: false},
		{loc: "weird://host/x.db", wantOK: false},
		// Driver schemes are not bare file paths; scheme match is
		// case-insensitive.
		{loc: "sqlite3:sakila.db", wantOK: false},
		{loc: "SQLITE3:sakila.db", wantOK: false},
		{loc: "sqlite:sakila.db", wantOK: false},
		{loc: `sqlite3:C:\db`, wantOK: false},
		{loc: "duckdb:foo.duckdb", wantOK: false},
		// A known scheme with a malformed single slash is still not a path.
		{loc: "sqlite3:/path/to/file", wantOK: false},
		// Well-formed placeholder: resolves at use time, not a path.
		{loc: "${env:DB_PATH}", wantOK: false},
		// Malformed placeholder syntax: also excluded.
		{loc: "sakila${env:.db", wantOK: false},
	}
	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			_, ok := isFpath(tc.loc)
			require.Equal(t, tc.wantOK, ok)
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
		{
			// Password-only userinfo on a scheme dburl can't parse:
			// the best-effort regex must mask it even with no username.
			name:      "password-only userinfo, unknown scheme",
			loc:       "mysqlx://:hunter2@host:33060",
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
