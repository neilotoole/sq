package ast

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInspector_findTableSegments(t *testing.T) {
	const q1 = `@mydb1 | .user | .uid, .username`

	ast, err := buildInitialAST(t, q1)
	require.Nil(t, err)
	err = NewWalker(ast).AddVisitor(typeSelectorNode, narrowTblSel).Walk()
	require.Nil(t, err)

	insp := NewInspector(ast)

	segs := ast.Segments()
	require.Equal(t, 3, len(segs))

	selSegs := insp.FindTableSegments()
	require.Equal(t, 1, len(selSegs), "should be 1 table segment: the tbl sel segment")
	finalSelSeg, err := insp.FindFinalTableSegment()
	require.Nil(t, err)
	require.Equal(t, selSegs[0], finalSelSeg)
}
