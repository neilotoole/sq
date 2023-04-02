package ast

import (
	"testing"

	"github.com/neilotoole/slogt"

	"github.com/stretchr/testify/require"
)

func TestInspector_findSelectableSegments(t *testing.T) {
	log := slogt.New(t)

	//  `@mydb1 | .user | .uid, .username`
	ast, err := buildInitialAST(t, fixtSelect1)
	require.Nil(t, err)
	err = NewWalker(log, ast).AddVisitor(typeSelectorNode, narrowTblSel).Walk()
	require.Nil(t, err)

	insp := NewInspector(log, ast)

	segs := ast.Segments()
	require.Equal(t, 3, len(segs))

	selSegs := insp.FindTablerSegments()
	require.Equal(t, 1, len(selSegs), "should be 1 selectable segment: the tbl sel segment")
	finalSelSeg, err := insp.FindFinalTablerSegment()
	require.Nil(t, err)
	require.Equal(t, selSegs[0], finalSelSeg)

	// `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`
	ast, err = buildInitialAST(t, fixtJoinQuery1)
	require.Nil(t, err)
	err = NewWalker(log, ast).AddVisitor(typeSelectorNode, narrowTblSel).Walk()
	require.Nil(t, err)
	insp = NewInspector(log, ast)

	segs = ast.Segments()
	require.Equal(t, 4, len(segs))

	selSegs = insp.FindTablerSegments()
	require.Equal(t, 2, len(selSegs), "should be 2 selectable segments: the tbl selector segment, and the join segment")

	finalSelSeg, err = insp.FindFinalTablerSegment()
	require.Nil(t, err)
	require.Equal(t, selSegs[1], finalSelSeg)
}
