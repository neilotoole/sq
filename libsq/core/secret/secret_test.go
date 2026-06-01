package secret_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
)

type stubResolver struct {
	values map[string]string
}

func (s *stubResolver) Resolve(_ context.Context, path string) (string, error) {
	v, ok := s.values[path]
	if !ok {
		return "", secret.ErrNotFound
	}
	return v, nil
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	reg := secret.NewRegistry()
	reg.Register("keyring", &stubResolver{values: map[string]string{"x": "y"}})

	v, err := reg.ResolveScheme(context.Background(), "keyring", "x")
	require.NoError(t, err)
	require.Equal(t, "y", v)

	_, err = reg.ResolveScheme(context.Background(), "unknown", "x")
	require.ErrorIs(t, err, secret.ErrUnknownScheme)
}

func TestContextRoundTrip(t *testing.T) {
	reg := secret.NewRegistry()
	ctx := secret.NewContext(context.Background(), reg)
	require.Same(t, reg, secret.FromContext(ctx))

	// No registry on plain context → nil.
	require.Nil(t, secret.FromContext(context.Background()))
}
