package render

import (
	"log/slog"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/vector"
	"github.com/neilotoole/sq/libsq/driver/dialect"
)

func TestDoSelectCols_ResultColumnVectors(t *testing.T) {
	vt := vector.Type{Dim: 4, Elem: vector.F32, Metric: "cosine"}
	hints := map[string]*TableHints{
		"t": {
			Vectors: map[string]vector.Type{"embedding": vt},
			Columns: []string{"id", "embedding", "note"},
		},
	}

	testCases := []struct {
		name        string
		slq         string
		vectorTable string
		want        map[int]vector.Type
	}{
		{name: "star", slq: `.t`, vectorTable: "t", want: map[int]vector.Type{1: vt}},
		{name: "cols", slq: `.t | .id, .embedding`, vectorTable: "t", want: map[int]vector.Type{1: vt}},
		{name: "alias", slq: `.t | .embedding:e, .id`, vectorTable: "t", want: map[int]vector.Type{0: vt}},
		{name: "case", slq: `.t | .EMBEDDING`, vectorTable: "t", want: map[int]vector.Type{0: vt}},
		{name: "tbl_col", slq: `.t | .t.embedding`, vectorTable: "t", want: map[int]vector.Type{0: vt}},
		{name: "no_vector_col", slq: `.t | .id, .note`, vectorTable: "t"},
		{name: "no_vector_table", slq: `.t | .embedding`},
		{name: "no_vector_table_star", slq: `.t`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a, err := ast.Parse(slog.Default(), tc.slq)
			require.NoError(t, err)

			var cols []ast.ResultColumn
			seg, err := ast.NewInspector(a).FindColExprSegment()
			require.NoError(t, err)
			if seg != nil {
				var ok bool
				cols, ok = langz.TypedSlice[ast.ResultColumn](seg.Children()...)
				require.True(t, ok)
			}

			rc := &Context{
				Dialect:     dialect.Dialect{Enquote: strconv.Quote},
				TableHints:  hints,
				VectorTable: tc.vectorTable,
			}
			_, err = doSelectCols(rc, cols)
			require.NoError(t, err)
			require.Equal(t, tc.want, rc.ResultColumnVectors)
		})
	}
}
