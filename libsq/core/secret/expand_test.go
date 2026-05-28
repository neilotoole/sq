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
