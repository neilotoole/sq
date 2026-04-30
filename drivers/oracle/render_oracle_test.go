package oracle

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
)

func TestRenderRowRange_nil(t *testing.T) {
	t.Parallel()
	got, err := renderRowRange(nil, nil)
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func TestRenderRowRange_empty(t *testing.T) {
	t.Parallel()
	got, err := renderRowRange(nil, &ast.RowRangeNode{Offset: -1, Limit: -1})
	require.NoError(t, err)
	require.Equal(t, "", got)
}

func TestRenderRowRange_offsetOnly(t *testing.T) {
	t.Parallel()
	got, err := renderRowRange(nil, &ast.RowRangeNode{Offset: 10, Limit: -1})
	require.NoError(t, err)
	require.Equal(t, "OFFSET 10 ROWS", got)
}

func TestRenderRowRange_limitOnly(t *testing.T) {
	t.Parallel()
	got, err := renderRowRange(nil, &ast.RowRangeNode{Offset: 0, Limit: 5})
	require.NoError(t, err)
	require.Equal(t, "OFFSET 0 ROWS FETCH NEXT 5 ROWS ONLY", got)
}

func TestRenderRowRange_offsetAndLimit(t *testing.T) {
	t.Parallel()
	got, err := renderRowRange(nil, &ast.RowRangeNode{Offset: 3, Limit: 7})
	require.NoError(t, err)
	require.Equal(t, "OFFSET 3 ROWS FETCH NEXT 7 ROWS ONLY", got)
}

func TestPreRenderOracle_injectsOrderByWhenRange(t *testing.T) {
	t.Parallel()
	f := &render.Fragments{Range: "OFFSET 1 ROWS FETCH NEXT 2 ROWS ONLY"}
	require.NoError(t, preRenderOracle(nil, f))
	require.Equal(t, "ORDER BY (SELECT 0 FROM DUAL)", f.OrderBy)
}

func TestPreRenderOracle_preservesExistingOrderBy(t *testing.T) {
	t.Parallel()
	f := &render.Fragments{
		Range:   "OFFSET 1 ROWS FETCH NEXT 2 ROWS ONLY",
		OrderBy: "ORDER BY \"ID\"",
	}
	require.NoError(t, preRenderOracle(nil, f))
	require.Equal(t, "ORDER BY \"ID\"", f.OrderBy)
}

func TestPreRenderOracle_noOpWithoutRange(t *testing.T) {
	t.Parallel()
	f := &render.Fragments{OrderBy: ""}
	require.NoError(t, preRenderOracle(nil, f))
	require.Equal(t, "", f.OrderBy)
}
