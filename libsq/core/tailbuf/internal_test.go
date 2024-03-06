package tailbuf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// InternalWindow exposes Buf's internal window for testing.
func InternalWindow[T any](b *Buf[T]) []T {
	return b.window
}

// TailNewSlice exposes Buf's internal tailNewSlice for testing.
func TailNewSlice[T any](b *Buf[T]) []T {
	return b.tailNewSlice()
}

// RequireEqualInternalState asserts that a and b have the same internal state.
func RequireEqualInternalState[T any](tb testing.TB, a, b *Buf[T]) {
	tb.Helper()
	require.Equal(tb, a.window, b.window)
	require.Equal(tb, a.len, b.len)
	require.Equal(tb, a.back, b.back)
	require.Equal(tb, a.front, b.front)
	require.Equal(tb, cap(a.window), cap(b.window))
	require.Equal(tb, len(a.window), len(b.window))
	require.Equal(tb, a.window, b.window)
}
