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
		Location: "postgres://alice:${keyring:my_db_pw}@db/sakila",
	}

	resolved, err := driver.ResolveSourceSecrets(ctx, src)
	require.NoError(t, err)
	require.NotSame(t, src, resolved, "must return a clone")
	require.Equal(t,
		"postgres://alice:hunter2@db/sakila",
		resolved.Location)
	require.Equal(t,
		"postgres://alice:${keyring:my_db_pw}@db/sakila",
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
		Location: "postgres://alice:${keyring:my_db_pw}@db/sakila",
	}
	// Placeholders present but no secret.Registry on context: must
	// return an explicit error rather than silently passing the
	// unresolved Location through to the driver, where it would
	// surface as a confusing DSN-parse or connection error.
	resolved, err := driver.ResolveSourceSecrets(context.Background(), src)
	require.Error(t, err)
	require.Nil(t, resolved)
	require.Contains(t, err.Error(), "@sakila")
	require.Contains(t, err.Error(), "no secret registry bound to context")
}

func TestGrips_ResolveSourceSecrets_NilSource(t *testing.T) {
	got, err := driver.ResolveSourceSecrets(context.Background(), nil)
	require.NoError(t, err)
	require.Nil(t, got)
}
