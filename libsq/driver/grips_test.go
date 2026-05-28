package driver_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

type captureResolver struct {
	value string
	calls []string
}

func (c *captureResolver) Resolve(_ context.Context, path string) (string, error) {
	c.calls = append(c.calls, path)
	return c.value, nil
}

func TestGrips_ResolveSourceSecrets(t *testing.T) {
	reg := secret.NewRegistry()
	reg.Register("keyring", &captureResolver{value: "hunter2"})
	ctx := secret.NewContext(context.Background(), reg)

	src := &source.Source{
		Handle:   "@sakila",
		Location: "postgres://alice:${keyring:@sakila/password}@db/sakila",
	}

	resolved, err := driver.ResolveSourceSecrets(ctx, src)
	require.NoError(t, err)
	require.NotSame(t, src, resolved, "must return a clone")
	require.Equal(t,
		"postgres://alice:hunter2@db/sakila",
		resolved.Location)
	require.Equal(t,
		"postgres://alice:${keyring:@sakila/password}@db/sakila",
		src.Location, "original src must be untouched")
}

func TestGrips_ResolveSourceSecrets_NoPlaceholder(t *testing.T) {
	reg := secret.NewRegistry()
	ctx := secret.NewContext(context.Background(), reg)
	src := &source.Source{
		Handle:   "@sakila",
		Location: "postgres://alice:hunter2@db/sakila",
	}
	resolved, err := driver.ResolveSourceSecrets(ctx, src)
	require.NoError(t, err)
	require.Same(t, src, resolved, "no placeholder => return input unchanged")
}

func TestGrips_ResolveSourceSecrets_NoRegistry(t *testing.T) {
	src := &source.Source{
		Handle:   "@sakila",
		Location: "postgres://alice:${keyring:@sakila/password}@db/sakila",
	}
	// No registry on context — placeholders are left intact, no error.
	resolved, err := driver.ResolveSourceSecrets(context.Background(), src)
	require.NoError(t, err)
	require.Same(t, src, resolved)
}

func TestGrips_ResolveSourceSecrets_NilSource(t *testing.T) {
	got, err := driver.ResolveSourceSecrets(context.Background(), nil)
	require.NoError(t, err)
	require.Nil(t, got)
}
