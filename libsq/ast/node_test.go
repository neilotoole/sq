package ast

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/lg/testlg"
)

func TestChildIndex(t *testing.T) {
	log := testlg.New(t).Strict(true)

	// `@mydb1 | .user, .address | join(.uid == .uid) | .uid, .username, .country`
	p := getSLQParser(fixtJoinQuery1)
	query := p.Query()
	ast, err := buildAst(log, query)
	require.Nil(t, err)
	require.NotNil(t, ast)
	require.Equal(t, 4, len(ast.Segments()))

	for i, seg := range ast.Segments() {
		index := nodeChildIndex(ast, seg)
		require.Equal(t, i, index)
	}
}

func TestNodesWithType(t *testing.T) {
	nodes := []Node{&ColSelector{}, &ColSelector{}, &TblSelector{}, &RowRange{}}

	require.Equal(t, 2, len(nodesWithType(nodes, typeColSelector)))
	require.Equal(t, 1, len(nodesWithType(nodes, typeTblSelector)))
	require.Equal(t, 1, len(nodesWithType(nodes, typeRowRange)))
	require.Equal(t, 0, len(nodesWithType(nodes, typeJoin)))
}

func TestAvg(t *testing.T) {
	const input = `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country | .[0:2] | avg(.uid)` //nolint:lll
	ast := mustParse(t, input)
	require.NotNil(t, ast)
}
