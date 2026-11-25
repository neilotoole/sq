package diffdoc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
)

// TestHunkRange pins the unified-diff range forms, matching GNU diff: a count
// of 1 is just the start line; a count of 0 uses the line before the empty
// range as its start; any other count is "start,count".
func TestHunkRange(t *testing.T) {
	testCases := []struct {
		start, count int
		want         string
	}{
		{start: 9, count: 1, want: "9"},
		{start: 9, count: 3, want: "9,3"},
		{start: 1, count: 2, want: "1,2"},
		// Empty range at line 2: the change goes after line 1.
		{start: 2, count: 0, want: "1,0"},
		// Empty range at the top (or an empty side).
		{start: 1, count: 0, want: "0,0"},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.want, diffdoc.HunkRange(tc.start, tc.count), "HunkRange(%d, %d)", tc.start, tc.count)
	}
}
