package secret_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
)

// identityFn returns the path unchanged. Useful for "rebuild but
// don't actually rewrite anything" tests.
func identityFn(_ context.Context, _, path string) (string, error) {
	return path, nil
}

// uppercaseFn rewrites every path to its uppercase form. Cheap,
// deterministic, and visually distinct in failure messages.
func uppercaseFn(_ context.Context, _, path string) (string, error) {
	return strings.ToUpper(path), nil
}

func TestRewritePlaceholders_NoPlaceholders(t *testing.T) {
	const in = "postgres://alice:hunter2@db/sakila"
	got, err := secret.RewritePlaceholders(context.Background(), in, uppercaseFn)
	require.NoError(t, err)
	require.Equal(t, in, got, "no placeholders → input returned verbatim")
}

func TestRewritePlaceholders_SinglePlaceholder(t *testing.T) {
	in := "postgres://alice:${keyring:abc}@db/sakila"
	got, err := secret.RewritePlaceholders(context.Background(), in, uppercaseFn)
	require.NoError(t, err)
	require.Equal(t, "postgres://alice:${keyring:ABC}@db/sakila", got)
}

func TestRewritePlaceholders_MultiplePlaceholders(t *testing.T) {
	in := "postgres://${env:user}:${keyring:abc}@${file:/host}/sakila"
	got, err := secret.RewritePlaceholders(context.Background(), in, uppercaseFn)
	require.NoError(t, err)
	require.Equal(t, "postgres://${env:USER}:${keyring:ABC}@${file:/HOST}/sakila", got)
}

func TestRewritePlaceholders_IdentityIsRoundTrip(t *testing.T) {
	// Identity rewriter should produce a byte-identical output, so a
	// "no-op" caller never accidentally mutates the template.
	in := "${keyring:a}-foo-${env:B}-bar-${file:/c}"
	got, err := secret.RewritePlaceholders(context.Background(), in, identityFn)
	require.NoError(t, err)
	require.Equal(t, in, got)
}

func TestRewritePlaceholders_SchemeAvailableToFn(t *testing.T) {
	// fn is only called on placeholder bodies; both scheme and path
	// must be passed through so the caller can decide what to rewrite.
	var seen []string
	_, err := secret.RewritePlaceholders(context.Background(),
		"${keyring:x}-${env:Y}",
		func(_ context.Context, scheme, path string) (string, error) {
			seen = append(seen, scheme+":"+path)
			return path, nil
		})
	require.NoError(t, err)
	require.Equal(t, []string{"keyring:x", "env:Y"}, seen)
}

func TestRewritePlaceholders_FnErrorPropagates(t *testing.T) {
	wantErr := errors.New("rewriter failed")
	_, err := secret.RewritePlaceholders(context.Background(),
		"${keyring:abc}",
		func(_ context.Context, _, _ string) (string, error) { return "", wantErr })
	require.Error(t, err)
	require.ErrorIs(t, err, wantErr)
	require.Contains(t, err.Error(), "${keyring:abc}",
		"error should include the offending placeholder for debugging")
}

func TestRewritePlaceholders_MalformedTemplatePropagates(t *testing.T) {
	// Unclosed ${ — findPlaceholders should reject it; the rewriter
	// surfaces the same parse error.
	_, err := secret.RewritePlaceholders(context.Background(),
		"postgres://alice:${keyring:abc@db/sakila",
		identityFn)
	require.Error(t, err)
}

func TestRewritePlaceholders_DollarEscapePreserved(t *testing.T) {
	// $$ is an escape for a literal $; rewriting must leave it intact.
	in := "literal $$ then ${keyring:x}"
	got, err := secret.RewritePlaceholders(context.Background(), in, uppercaseFn)
	require.NoError(t, err)
	require.Equal(t, "literal $$ then ${keyring:X}", got)
}

func TestRewritePlaceholders_NewPathCanContainSpecialChars(t *testing.T) {
	// Rewritten paths can contain '/', ':', '@', and other characters
	// that would normally be URL-significant — they sit inside the
	// ${...} body and are passed through verbatim.
	got, err := secret.RewritePlaceholders(context.Background(),
		"${file:./foo}",
		func(_ context.Context, _, _ string) (string, error) {
			return "/abs/with:colon/and@at", nil
		})
	require.NoError(t, err)
	require.Equal(t, "${file:/abs/with:colon/and@at}", got)
}

// stringPathRewriter is a convenience helper used by other tests in
// this package; included here as a compile-time check that the
// expected signature matches.
var _ = func() func(context.Context, string, string) (string, error) {
	return func(_ context.Context, scheme, path string) (string, error) {
		return fmt.Sprintf("%s:%s", scheme, path), nil
	}
}
