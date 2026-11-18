package oracle

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestColumnsFromRows(t *testing.T) {
	t.Run("regroups_rows_into_columns", func(t *testing.T) {
		args := []any{1, "a", nil, 2, "b", 2.5, 3, nil, 3.5}
		cols, err := columnsFromRows(args, 3)
		require.NoError(t, err)
		require.Equal(t, [][]any{
			{1, 2, 3},
			{"a", "b", nil},
			{nil, 2.5, 3.5},
		}, cols)
	})

	t.Run("single_row", func(t *testing.T) {
		cols, err := columnsFromRows([]any{1, "a"}, 2)
		require.NoError(t, err)
		require.Equal(t, [][]any{{1}, {"a"}}, cols)
	})

	t.Run("bad_args", func(t *testing.T) {
		_, err := columnsFromRows(nil, 2)
		require.Error(t, err)

		_, err = columnsFromRows([]any{1, 2, 3}, 2)
		require.Error(t, err, "args not a multiple of numCols")

		_, err = columnsFromRows([]any{1}, 0)
		require.Error(t, err)
	})
}

func TestArrayBindable(t *testing.T) {
	now := time.Now()
	testCases := []struct {
		name string
		cols [][]any
		want bool
	}{
		{
			name: "uniform",
			cols: [][]any{{int64(1), int64(2)}, {"a", "b"}, {now, now}, {[]byte("x"), []byte("y")}},
			want: true,
		},
		{
			name: "nulls_anywhere",
			cols: [][]any{{nil, int64(2), nil}, {"a", nil, "c"}},
			want: true,
		},
		{
			name: "all_null",
			cols: [][]any{{nil, nil}},
			want: true,
		},
		{
			name: "mixed_go_types",
			cols: [][]any{{int64(1), float64(2.5)}},
			want: false,
		},
		{
			name: "string_at_limit",
			cols: [][]any{{strings.Repeat("a", maxArrayBindString), "b"}},
			want: true,
		},
		{
			name: "string_over_limit",
			cols: [][]any{{strings.Repeat("a", maxArrayBindString+1), "b"}},
			want: false,
		},
		{
			name: "bytes_at_limit",
			cols: [][]any{{make([]byte, maxArrayBindBytes), []byte("b")}},
			want: true,
		},
		{
			name: "bytes_over_limit",
			cols: [][]any{{make([]byte, maxArrayBindBytes+1), []byte("b")}},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, arrayBindable(tc.cols))
		})
	}
}
