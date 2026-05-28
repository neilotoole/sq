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
