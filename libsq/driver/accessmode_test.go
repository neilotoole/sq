package driver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessMode_suffix(t *testing.T) {
	require.Equal(t, "rw", ModeReadWrite.suffix())
	require.Equal(t, "ro", ModeReadOnly.suffix())
	require.Equal(t, "rox", ModeReadOnlyExplicit.suffix())
	// suffixes must be distinct so the three modes get distinct cache keys.
	require.NotEqual(t, ModeReadOnly.suffix(), ModeReadOnlyExplicit.suffix())
}

// TestWithMode_RoundTrip covers the ctx carrier retained in Approach 1a
// for the verifySourceCatalogSchema bypass. Drivers no longer read ctx
// (they take an explicit mode parameter); these helpers serve only that
// bypass path.
func TestWithMode_RoundTrip(t *testing.T) {
	ctx := context.Background()
	require.False(t, IsReadOnly(ctx))
	require.False(t, IsReadOnlyExplicit(ctx))

	// ModeReadWrite must not write a value (so a bare ctx and a
	// RW-marked ctx are indistinguishable).
	require.False(t, IsReadOnly(WithMode(ctx, ModeReadWrite)))

	roCtx := WithMode(ctx, ModeReadOnly)
	require.True(t, IsReadOnly(roCtx))
	require.False(t, IsReadOnlyExplicit(roCtx))

	roxCtx := WithMode(ctx, ModeReadOnlyExplicit)
	require.True(t, IsReadOnly(roxCtx))
	require.True(t, IsReadOnlyExplicit(roxCtx))
}

type testCtxKey struct{}

func TestWithMode_SurvivesChildContext(t *testing.T) {
	ctx := WithMode(context.Background(), ModeReadOnlyExplicit)
	child := context.WithValue(ctx, testCtxKey{}, "x")
	require.True(t, IsReadOnlyExplicit(child),
		"read-only marker must survive context derivation")
}
