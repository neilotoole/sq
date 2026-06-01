package secret_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
)

func newReg(t *testing.T, values map[string]string) *secret.Registry {
	t.Helper()
	reg := secret.NewRegistry()
	reg.Register("keyring", &stubResolver{values: values})
	return reg
}

func TestExpand_NoPlaceholders(t *testing.T) {
	reg := newReg(t, nil)
	got, err := reg.Expand(context.Background(), "postgres://alice:hunter2@db/sakila")
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:hunter2@db/sakila", got)
}

func TestExpand_DollarEscape(t *testing.T) {
	reg := newReg(t, nil)
	got, err := reg.Expand(context.Background(), "abc$$def")
	require.NoError(t, err)
	require.Equal(t, "abc$def", got)
}

func TestExpand_NonURLWholeValue(t *testing.T) {
	reg := newReg(t, map[string]string{"x": "secret-value"})
	got, err := reg.Expand(context.Background(), "${keyring:x}")
	require.NoError(t, err)
	require.Equal(t, "secret-value", got)
}

func TestExpand_UnknownScheme(t *testing.T) {
	reg := newReg(t, nil)
	_, err := reg.Expand(context.Background(), "${vault:foo}")
	require.ErrorIs(t, err, secret.ErrUnknownScheme)
}

func TestExpand_MissingSecret(t *testing.T) {
	reg := newReg(t, nil)
	_, err := reg.Expand(context.Background(), "${keyring:nope}")
	require.ErrorIs(t, err, secret.ErrNotFound)
	// keyring-scheme not-found should carry the recovery hint so the
	// user can copy-paste the fix command from the error. Anchored on
	// "(run: " to pin flag order; shell-safety is exercised by
	// TestExpand_MissingSecret_HintShellSafe.
	require.Contains(t, err.Error(), `(run: sq config keyring create -p -- 'nope')`)
}

// TestExpand_MissingSecret_HintShellSafe pins the shell-safe shape of
// the recovery hint for keyring paths that would otherwise break naive
// copy-paste — or worse, execute attacker-controlled shell when pasted.
// The placeholder grammar in parse.go permits any byte except a closing
// brace in a path, so all bash metacharacters are reachable. POSIX
// single-quoting neutralizes every case; "--" stops flag parsing for
// "-"-prefixed paths.
func TestExpand_MissingSecret_HintShellSafe(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string // full parenthesized hint the error must contain
	}{
		{
			name: "path with space",
			path: "sakila db pw",
			want: `(run: sq config keyring create -p -- 'sakila db pw')`,
		},
		{
			name: "path with leading dash",
			path: "-looks-like-flag",
			want: `(run: sq config keyring create -p -- '-looks-like-flag')`,
		},
		{
			name: "path with command substitution must not execute",
			path: "$(whoami)",
			want: `(run: sq config keyring create -p -- '$(whoami)')`,
		},
		{
			name: "path with backticks must not execute",
			path: "`whoami`",
			want: "(run: sq config keyring create -p -- '`whoami`')",
		},
		{
			name: "path with variable expansion must not expand",
			path: "$HOME/db",
			want: `(run: sq config keyring create -p -- '$HOME/db')`,
		},
		{
			name: "path with backslash",
			path: `a\b`,
			want: `(run: sq config keyring create -p -- 'a\b')`,
		},
		{
			name: "path with embedded double-quote",
			path: `has"quote`,
			want: `(run: sq config keyring create -p -- 'has"quote')`,
		},
		{
			name: "path with embedded single-quote escapes as '\\''",
			path: `it's`,
			want: `(run: sq config keyring create -p -- 'it'\''s')`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reg := newReg(t, nil)
			_, err := reg.Expand(context.Background(), "${keyring:"+tc.path+"}")
			require.ErrorIs(t, err, secret.ErrNotFound)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestExpand_MissingSecret_NonKeyringNoHint(t *testing.T) {
	// Non-keyring resolvers (env, file, future op:) don't get the
	// "sq config keyring create" hint — that command only writes to
	// the keyring scheme. Sanity-check by registering a stub under "env".
	reg := secret.NewRegistry()
	reg.Register("env", &stubResolver{values: nil})
	_, err := reg.Expand(context.Background(), "${env:MISSING}")
	require.ErrorIs(t, err, secret.ErrNotFound)
	require.NotContains(t, err.Error(), "sq config keyring create",
		"only keyring not-found should suggest the create command")
}

func TestExpand_MultiplePlaceholders(t *testing.T) {
	reg := newReg(t, map[string]string{"a": "AAA", "b": "BBB"})
	got, err := reg.Expand(context.Background(), "${keyring:a}-${keyring:b}-end")
	require.NoError(t, err)
	require.Equal(t, "AAA-BBB-end", got)
}

func TestExpand_ResolvedValueIsLiteral(t *testing.T) {
	// Resolved values are never re-scanned.
	reg := newReg(t, map[string]string{"x": "${keyring:other}"})
	got, err := reg.Expand(context.Background(), "${keyring:x}")
	require.NoError(t, err)
	require.Equal(t, "${keyring:other}", got)
}

func TestExpand_URLUserinfo_PasswordEncoded(t *testing.T) {
	reg := newReg(t, map[string]string{"pw": "p@ss:word/with#special?chars"})
	got, err := reg.Expand(context.Background(),
		"postgres://alice:${keyring:pw}@db/sakila")
	require.NoError(t, err)
	require.Equal(t,
		"postgres://alice:p%40ss%3Aword%2Fwith%23special%3Fchars@db/sakila",
		got)
}

func TestExpand_URLUserinfo_UsernameEncoded(t *testing.T) {
	reg := newReg(t, map[string]string{"u": "ali ce"})
	got, err := reg.Expand(context.Background(),
		"postgres://${keyring:u}:hunter2@db/sakila")
	require.NoError(t, err)
	require.Equal(t,
		"postgres://ali%20ce:hunter2@db/sakila",
		got)
}

func TestExpand_URLHost_NotUserinfo_NotEncoded(t *testing.T) {
	// A placeholder in the host position is NOT URL-userinfo and is not
	// re-encoded — it's the user's responsibility to provide a valid
	// hostname.
	reg := newReg(t, map[string]string{"h": "db.example.com"})
	got, err := reg.Expand(context.Background(),
		"postgres://alice:hunter2@${keyring:h}/sakila")
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:hunter2@db.example.com/sakila", got)
}

func TestExpand_WholeURLOpaque(t *testing.T) {
	// A whole-DSN placeholder: the resolved value IS the URL, no
	// userinfo encoding applies (encoding is the caller's job).
	reg := newReg(t, map[string]string{
		"dsn": "postgres://alice:p%40ss@db/sakila",
	})
	got, err := reg.Expand(context.Background(), "${keyring:dsn}")
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:p%40ss@db/sakila", got)
}

func TestExpand_NonURL_NoEncoding(t *testing.T) {
	// A non-URL template (e.g. a file path) is not parsed as URL and
	// resolved values are spliced raw.
	reg := newReg(t, map[string]string{"p": "/secrets/data with space.xlsx"})
	got, err := reg.Expand(context.Background(), "${keyring:p}")
	require.NoError(t, err)
	require.Equal(t, "/secrets/data with space.xlsx", got)
}

// TestExpand_PortPlaceholder_DoesNotBreakUserinfoEncoding regression-checks
// that a placeholder in the URL port position doesn't short-circuit
// userinfo detection for other placeholders in the same template. The
// previous implementation used a non-digit sentinel ("__SQ_SECRET_REF_N__")
// which url.Parse rejected when substituted into the port, causing the
// whole parse to fail and all userinfo placeholders to lose their
// URL-encoding pass.
func TestExpand_PortPlaceholder_DoesNotBreakUserinfoEncoding(t *testing.T) {
	reg := newReg(t, map[string]string{
		"pw":   `p@ss:word!`,
		"port": "5432",
	})
	got, err := reg.Expand(context.Background(),
		"postgres://alice:${keyring:pw}@host:${keyring:port}/db")
	require.NoError(t, err)
	// The password must be URL-encoded (userinfo splice); the port must
	// land raw. A regression would either leave the password unencoded
	// or break the URL entirely.
	require.Equal(t,
		"postgres://alice:p%40ss%3Aword%21@host:5432/db",
		got)
}

// TestExpand_UserinfoSentinelLiteralCollision exercises the (extremely
// unlikely) case where user-supplied URL data legitimately contains
// the digit-only sentinel format used by userinfoPlaceholders. The
// format is "9999000<7-digit index>9999" — 18 fixed-position digits.
// A collision would mis-classify an unrelated placeholder as residing
// in userinfo and apply URL encoding spuriously.
//
// Unlike libsq/source/location/pickSentinels, userinfoPlaceholders
// uses a fixed prefix (no salt bumping) on the principle that an 18-
// digit fixed-position run is unlikely enough to dispense with the
// salt machinery. This test pins that decision: a literal sentinel
// substring in user data still produces correctly-encoded output.
func TestExpand_UserinfoSentinelLiteralCollision(t *testing.T) {
	// Build a URL where the path component contains the literal
	// sentinel for placeholder index 0 ("999900000000009999"). If the
	// detection logic confuses the literal for the sentinel, the
	// password placeholder would be flagged as NOT in userinfo and
	// would skip URL encoding — leaking a special char as a raw `@`.
	reg := newReg(t, map[string]string{"pw": "p@ss"})
	got, err := reg.Expand(context.Background(),
		"postgres://alice:${keyring:pw}@host/db/999900000000009999")
	require.NoError(t, err)
	// The password must still be encoded (@ -> %40) AND the path
	// substring must survive unchanged.
	require.Contains(t, got, "alice:p%40ss@host",
		"password must be URL-encoded despite literal sentinel in path")
	require.Contains(t, got, "/999900000000009999",
		"literal-sentinel path substring must survive unchanged")
}
