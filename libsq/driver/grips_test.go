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

// TestGrips_ResolveSourceSecrets_NoRefs_Unescape verifies that the $$
// escape is honored even when the location contains no ${scheme:path}
// refs: the driver must receive the literal form. This is what makes
// the v0.54.0 config upgrade's escaping of legacy locations (which
// never contain intentional placeholders) connect byte-identically.
// No secret.Registry is bound to the context: unescaping must not
// require one, since a refless location resolves nothing.
func TestGrips_ResolveSourceSecrets_NoRefs_Unescape(t *testing.T) {
	tests := []struct {
		name string
		loc  string
		want string
	}{
		{
			name: "escaped dollar in password",
			loc:  "postgres://alice:p$$ss@db/sakila",
			want: "postgres://alice:p$ss@db/sakila",
		},
		{
			name: "escaped well-formed placeholder",
			loc:  "postgres://alice:$${env:HOME}@db/sakila",
			want: "postgres://alice:${env:HOME}@db/sakila",
		},
		{
			name: "escaped malformed placeholder",
			loc:  "postgres://alice:p$${ss}w@db/sakila",
			want: "postgres://alice:p${ss}w@db/sakila",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := &source.Source{Handle: "@sakila", Location: tc.loc}
			resolved, err := driver.ResolveSourceSecrets(context.Background(), src)
			require.NoError(t, err)
			require.NotSame(t, src, resolved, "must return a clone when location changes")
			require.Equal(t, tc.want, resolved.Location)
			require.Equal(t, tc.loc, src.Location, "original src must be untouched")
		})
	}
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
