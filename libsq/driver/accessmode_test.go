package driver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveMode(t *testing.T) {
	require.Equal(t, ModeReadWrite, resolveMode(nil))
	require.Equal(t, ModeReadWrite, resolveMode([]OpenOpt{}))
	require.Equal(t, ModeReadOnly, resolveMode([]OpenOpt{ReadOnly()}))
	require.Equal(t, ModeReadOnlyExplicit, resolveMode([]OpenOpt{ReadOnlyExplicit()}))
	require.Equal(t, ModeReadOnlyExplicit, resolveMode([]OpenOpt{Mode(ModeReadOnlyExplicit)}))

	// ReadOnly must not downgrade an explicit request, in either order.
	require.Equal(t, ModeReadOnlyExplicit,
		resolveMode([]OpenOpt{ReadOnlyExplicit(), ReadOnly()}))
	require.Equal(t, ModeReadOnlyExplicit,
		resolveMode([]OpenOpt{ReadOnly(), ReadOnlyExplicit()}))

	// nil opts are skipped.
	require.Equal(t, ModeReadOnly, resolveMode([]OpenOpt{nil, ReadOnly(), nil}))
}

func TestAccessMode_suffix(t *testing.T) {
	require.Equal(t, "rw", ModeReadWrite.suffix())
	require.Equal(t, "ro", ModeReadOnly.suffix())
	require.Equal(t, "rox", ModeReadOnlyExplicit.suffix())
	// suffixes must be distinct so the three modes get distinct cache keys.
	require.NotEqual(t, ModeReadOnly.suffix(), ModeReadOnlyExplicit.suffix())
}

func TestWithMode_RoundTrip(t *testing.T) {
	ctx := context.Background()
	require.False(t, IsReadOnly(ctx))
	require.False(t, IsReadOnlyExplicit(ctx))

	// ModeReadWrite must not write a value (so a bare ctx and a
	// RW-marked ctx are indistinguishable to drivers).
	require.False(t, IsReadOnly(WithMode(ctx, ModeReadWrite)))

	roCtx := WithMode(ctx, ModeReadOnly)
	require.True(t, IsReadOnly(roCtx))
	require.False(t, IsReadOnlyExplicit(roCtx))

	roxCtx := WithMode(ctx, ModeReadOnlyExplicit)
	require.True(t, IsReadOnly(roxCtx))
	require.True(t, IsReadOnlyExplicit(roxCtx))
}

func TestWithMode_SurvivesChildContext(t *testing.T) {
	ctx := WithMode(context.Background(), ModeReadOnlyExplicit)
	child := context.WithValue(ctx, struct{}{}, "x")
	require.True(t, IsReadOnlyExplicit(child),
		"read-only marker must survive context derivation")
}
