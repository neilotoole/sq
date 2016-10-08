package ql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

//func TestInspector_findSegments(t *testing.T) {
//
//	//  `@mydb1 | .user | .uid, .username`
//	ast, err := getAST(fixtSelect1)
//	require.Nil(t, err)
//	insp := NewInspector(ast)
//
//	segs := insp.findSegments()
//	require.Equal(t, 3, len(segs))
//
//	// `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`
//	ast, err = getAST(fixtJoinQuery1)
//	require.Nil(t, err)
//	insp = NewInspector(ast)
//
//	segs = insp.findSegments()
//	require.Equal(t, 4, len(segs))
//
//	// `@my1 |.user, .address | join(.uid) |  .[0:4] | .user.uid, .username, .country`
//	ast, err = getAST(fixtJoinRowRange)
//	require.Nil(t, err)
//	insp = NewInspector(ast)
//
//	segs = insp.findSegments()
//	require.Equal(t, 5, len(segs))
//
//}
func TestInspector_findSelectableSegments(t *testing.T) {

	//  `@mydb1 | .user | .uid, .username`
	ast, err := getAST(fixtSelect1)
	require.Nil(t, err)
	err = NewWalker(ast).AddVisitor(TypeSelector, narrowTblSel).Walk()
	require.Nil(t, err)

	insp := NewInspector(ast)

	segs := ast.Segments()
	require.Equal(t, 3, len(segs))

	selSegs := insp.findSelectableSegments()
	require.Equal(t, 1, len(selSegs), "should be 1 selectable segment: the tbl sel segment")
	finalSelSeg, err := insp.findFinalSelectableSegment()
	require.Nil(t, err)
	require.Equal(t, selSegs[0], finalSelSeg)

	//// `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country`
	ast, err = getAST(fixtJoinQuery1)
	require.Nil(t, err)
	err = NewWalker(ast).AddVisitor(TypeSelector, narrowTblSel).Walk()
	require.Nil(t, err)
	insp = NewInspector(ast)

	segs = ast.Segments()
	require.Equal(t, 4, len(segs))

	selSegs = insp.findSelectableSegments()
	require.Equal(t, 2, len(selSegs), "should be 2 selectable segments: the tbl selector segment, and the join segment")

	finalSelSeg, err = insp.findFinalSelectableSegment()
	require.Nil(t, err)
	require.Equal(t, selSegs[1], finalSelSeg)

}
