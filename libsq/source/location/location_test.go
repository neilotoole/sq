package location_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source/location"
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
		{loc: "duckdb:///path/to/duck.duckdb", want: true},
		{loc: "sqlserver://sq:p_ssW0rd@localhost?database=sqtest", want: true},
		{loc: "postgres://sq:p_ssW0rd@localhost/sqtest?sslmode=disable", want: true},
		{loc: "mysql://sq:p_ssW0rd@tcp(localhost:3306)/sqtest", want: true},
		{loc: "clickhouse://sakila:p_ssW0rd@localhost:9000/sakila", want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.loc, func(t *testing.T) {
			got := location.IsSQL(tc.loc)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestWithPassword(t *testing.T) {
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
			loc:  "clickhouse://sakila:p_ssW0rd@localhost:9000/sakila",
			pw:   "p_ssW0rd",
			want: "clickhouse://sakila:p_ssW0rd@localhost:9000/sakila",
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
		{
			// Empty password on a URL without userinfo: leave as-is
			// rather than emitting "postgres://@host/db".
			loc:  "postgres://localhost/sakila",
			pw:   "",
			want: "postgres://localhost/sakila",
		},
		{
			// Non-empty password but no username: reject.
			// "postgres://:hunter2@host/db" is rarely intentional.
			loc:     "postgres://localhost/sakila",
			pw:      "hunter2",
			wantErr: true,
		},
		{
			// Same case but with an empty-username userinfo block.
			loc:     "postgres://:oldpw@localhost/sakila",
			pw:      "hunter2",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			t.Parallel()

			beforeURL, err := url.ParseRequestURI(tc.loc)
			require.NoError(t, err)

			got, gotErr := location.WithPassword(tc.loc, tc.pw)
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

func TestShort(t *testing.T) {
	testCases := []struct {
		loc  string
		want string
	}{
		{loc: "/path/to/data.xlsx", want: "data.xlsx"},
		{loc: "sqlite3:///path/to/sqlite.db", want: "sqlite.db"},
		{loc: "postgres://sakila:p_ssW0rd@localhost:5432/sakila", want: "sakila@localhost:5432/sakila"},
		{loc: "mysql://sakila:p_ssW0rd@localhost:3306/sakila", want: "sakila@localhost:3306/sakila"},
		{loc: "rqlite://sakila:p_ssW0rd@localhost:4001", want: "sakila@localhost:4001"},
		{loc: "rqlite://sakila:p_ssW0rd@localhost:4001?tls=true", want: "sakila@localhost:4001"},
		{loc: "rqlite://localhost:4001", want: "localhost:4001"},
		{loc: "rqlite://sakila:p_ssW0rd@localhost:4001?level=strong", want: "sakila@localhost:4001"},
		// Unknown schemes must not leak inline credentials. rqlites:// was once
		// special-cased; now it flows through the generic redaction paths.
		{loc: "rqlites://alice:secret@host:4001", want: "rqlites://alice:xxxxx@host:4001"},
		{loc: "mysqlx://bob:hunter2@host:33060", want: "mysqlx://bob:xxxxx@host:33060"},
	}

	for _, tc := range testCases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			got := location.Short(tc.loc)
			require.NotContains(t, got, "p_ssW0rd",
				"Short must not echo passwords")
			require.NotContains(t, got, "secret",
				"Short must not echo passwords for unknown schemes")
			require.NotContains(t, got, "hunter2",
				"Short must not echo passwords for unknown schemes")
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseUnknownSchemeRedaction(t *testing.T) {
	// Parse must not echo inline passwords in error messages for
	// unknown schemes. rqlites:// is used as a representative case
	// (it was once special-cased; now it flows through generic paths).
	cases := []struct {
		loc      string
		password string
	}{
		{loc: "rqlites://alice:secret@host", password: "secret"},
		{loc: "mysqlx://bob:hunter2@host:33060", password: "hunter2"},
	}
	for _, tc := range cases {
		t.Run(tu.Name(tc.loc), func(t *testing.T) {
			_, err := location.Parse(tc.loc)
			require.Error(t, err)
			require.NotContains(t, err.Error(), tc.password,
				"Parse must not echo inline passwords on unknown schemes")
			require.Contains(t, err.Error(), "xxxxx",
				"redactBestEffort should mask the password")
		})
	}
}

func TestParseRqliteMalformedRedaction(t *testing.T) {
	// Malformed IPv6 bracket: url.ParseRequestURI rejects it, and the
	// error must not echo the inline password.
	_, err := location.Parse("rqlite://alice:secret@[::1")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret")
	require.Contains(t, err.Error(), "xxxxx")
}

func TestMergeQuery(t *testing.T) {
	testCases := []struct {
		name    string
		loc     string
		params  url.Values
		want    string
		wantErr bool
	}{
		{
			name:   "nil params returns loc unchanged",
			loc:    "rqlite://host:4001",
			params: nil,
			want:   "rqlite://host:4001",
		},
		{
			name:   "single param on bare loc",
			loc:    "rqlite://host:4001",
			params: url.Values{"tls": {"true"}},
			want:   "rqlite://host:4001?tls=true",
		},
		{
			name:   "existing unrelated params preserved",
			loc:    "rqlite://host:4001?level=strong",
			params: url.Values{"tls": {"true"}},
			want:   "rqlite://host:4001?level=strong&tls=true",
		},
		{
			name:   "existing same-key param replaced not duplicated",
			loc:    "rqlite://host:4001?tls=false",
			params: url.Values{"tls": {"true"}},
			want:   "rqlite://host:4001?tls=true",
		},
		{
			name:   "credentials round-trip unchanged",
			loc:    "rqlite://alice:pw@host:4001",
			params: url.Values{"tls": {"true"}},
			want:   "rqlite://alice:pw@host:4001?tls=true",
		},
		{
			name:    "unparseable loc errors without echoing it",
			loc:     "rqlite://alice:secret@[::1",
			params:  url.Values{"tls": {"true"}},
			wantErr: true,
		},
		{
			name:    "scheme-less loc rejected",
			loc:     "/path/to/file.db",
			params:  url.Values{"tls": {"true"}},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := location.MergeQuery(tc.loc, tc.params)
			if tc.wantErr {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "secret",
					"merge errors must not echo credentials")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestIsSecretQueryParam(t *testing.T) {
	testCases := []struct {
		key  string
		want bool
	}{
		{key: "", want: false},
		{key: "password", want: true},
		{key: "Password", want: true},
		{key: "passwd", want: true},
		{key: "pwd", want: true},
		{key: "pw", want: true},
		{key: "secret", want: true},
		{key: "token", want: true},
		{key: "apikey", want: true},
		{key: "api_key", want: true},
		{key: "access_token", want: true},
		{key: "auth_token", want: true},
		{key: "sslpassword", want: true},
		{key: "_auth_pass", want: true},
		{key: "client_secret", want: true},
		// Non-secrets, including near-misses.
		{key: "user", want: false},
		{key: "_auth_user", want: false},
		{key: "_auth_salt", want: false},
		{key: "database", want: false},
		{key: "sslmode", want: false},
		{key: "allowCleartextPasswords", want: false},
		{key: "allowNativePasswords", want: false},
		{key: "_foreign_keys", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			require.Equal(t, tc.want, location.IsSecretQueryParam(tc.key))
		})
	}
}

func TestStripSecrets(t *testing.T) {
	testCases := []struct {
		name string
		loc  string
		want string
	}{
		{
			name: "empty",
			loc:  "",
			want: "",
		},
		{
			name: "no secrets",
			loc:  "postgres://alice@localhost:5432/sakila?sslmode=require",
			want: "postgres://alice@localhost:5432/sakila?sslmode=require",
		},
		{
			name: "userinfo password dropped",
			loc:  "postgres://alice:hunter2@localhost:5432/sakila",
			want: "postgres://alice@localhost:5432/sakila",
		},
		{
			name: "userinfo password and secret query value",
			loc:  "postgres://alice:hunter2@localhost/sakila?sslpassword=abc&sslmode=require",
			want: "postgres://alice@localhost/sakila?sslpassword=&sslmode=require",
		},
		{
			name: "sqlite auth query params",
			loc:  "sqlite3:///data/app.db?_auth_user=admin&_auth_pass=hunter2",
			want: "sqlite3:///data/app.db?_auth_user=admin&_auth_pass=",
		},
		{
			name: "sqlserver query before path",
			loc:  "sqlserver://alice:hunter2@server:1433?database=sakila&password=abc",
			want: "sqlserver://alice@server:1433?database=sakila&password=",
		},
		{
			name: "empty secret value unchanged",
			loc:  "sqlite3:///data/app.db?_auth_pass=",
			want: "sqlite3:///data/app.db?_auth_pass=",
		},
		{
			name: "username only userinfo unchanged",
			loc:  "postgres://alice@localhost/sakila",
			want: "postgres://alice@localhost/sakila",
		},
		{
			name: "empty password dropped with colon",
			loc:  "postgres://alice:@localhost/sakila",
			want: "postgres://alice@localhost/sakila",
		},
		{
			name: "ipv6 host",
			loc:  "postgres://alice:hunter2@[::1]:5432/sakila",
			want: "postgres://alice@[::1]:5432/sakila",
		},
		{
			name: "placeholder password kept verbatim",
			loc:  "postgres://alice:${keyring:pg-prod}@localhost/sakila",
			want: "postgres://alice:${keyring:pg-prod}@localhost/sakila",
		},
		{
			name: "mixed literal and placeholder password dropped",
			loc:  "postgres://alice:abc${env:PGPASS}@localhost/sakila",
			want: "postgres://alice@localhost/sakila",
		},
		{
			name: "placeholder secret query value kept verbatim",
			loc:  "sqlite3:///data/app.db?_auth_user=admin&_auth_pass=${keyring:app-db}",
			want: "sqlite3:///data/app.db?_auth_user=admin&_auth_pass=${keyring:app-db}",
		},
		{
			name: "mixed literal and placeholder query value blanked",
			loc:  "sqlite3:///data/app.db?_auth_pass=abc${env:DBPASS}",
			want: "sqlite3:///data/app.db?_auth_pass=",
		},
		{
			name: "placeholder in non-secret position untouched",
			loc:  "postgres://alice@${env:PGHOST}/sakila?application_name=app1",
			want: "postgres://alice@${env:PGHOST}/sakila?application_name=app1",
		},
		{
			name: "no scheme query still scrubbed",
			loc:  "/data/app.db?_auth_pass=hunter2",
			want: "/data/app.db?_auth_pass=",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, location.StripSecrets(tc.loc))
		})
	}
}
