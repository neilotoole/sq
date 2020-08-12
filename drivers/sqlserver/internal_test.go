package sqlserver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlaceholders(t *testing.T) {
	testCases := []struct {
		numCols int
		numRows int
		want    string
	}{
		{numCols: 0, numRows: 0, want: ""},
		{numCols: 1, numRows: 1, want: "(@p1)"},
		{numCols: 2, numRows: 1, want: "(@p1, @p2)"},
		{numCols: 1, numRows: 2, want: "(@p1), (@p2)"},
		{numCols: 2, numRows: 2, want: "(@p1, @p2), (@p3, @p4)"},
	}

	for _, tc := range testCases {
		got := placeholders(tc.numCols, tc.numRows)
		require.Equal(t, tc.want, got)
	}
}
