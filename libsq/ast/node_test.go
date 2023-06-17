package ast

import (
	"testing"

	"github.com/neilotoole/slogt"

	"github.com/stretchr/testify/require"
)

func TestChildIndex(t *testing.T) {
	log := slogt.New(t)

	// `@mydb1 | .user, .address | join(.uid == .uid) | .uid, .username, .country`
	p := getSLQParser(fixtJoinQuery1)
	query := p.Query()
	ast, err := buildAST(log, query)
	require.Nil(t, err)
	require.NotNil(t, ast)
	require.Equal(t, 4, len(ast.Segments()))

	for i, seg := range ast.Segments() {
		index := nodeChildIndex(ast, seg)
		require.Equal(t, i, index)
	}
}

func TestNodesWithType(t *testing.T) {
	nodes := []Node{&ColSelectorNode{}, &ColSelectorNode{}, &TblSelectorNode{}, &RowRangeNode{}}

	require.Equal(t, 2, len(nodesWithType(nodes, typeColSelectorNode)))
	require.Equal(t, 1, len(nodesWithType(nodes, typeTblSelectorNode)))
	require.Equal(t, 1, len(nodesWithType(nodes, typeRowRangeNode)))
	require.Equal(t, 0, len(nodesWithType(nodes, typeJoinNode)))
}

func TestAvg(t *testing.T) {
	const input = `@mydb1 | .user, .address | join(.user.uid == .address.uid) | .uid, .username, .country | .[0:2] | avg(.uid)` //nolint:lll
	ast := mustParse(t, input)
	require.NotNil(t, ast)
}

func TestNodePrevNextSibling(t *testing.T) {
	const in = `@sakila | .actor | .actor_id == 2`

	log := slogt.New(t)

	a, err := Parse(log, in)
	require.NoError(t, err)

	equalsNode := NodesHavingText(a, "==")[0]

	gotPrev := NodePrevSibling(equalsNode)
	require.Equal(t, ".actor_id", gotPrev.Text())
	require.Nil(t, NodePrevSibling(gotPrev))

	gotNext := NodeNextSibling(equalsNode)
	require.Equal(t, "2", gotNext.Text())
	require.Nil(t, NodeNextSibling(gotNext))
}

func TestNodeUnwrap(t *testing.T) {
	var ok bool
	exprA := &ExprNode{}
	exprB := &ExprNode{}

	var gotExpr *ExprNode

	gotExpr, ok = NodeUnwrap[*ExprNode](exprA)
	require.True(t, ok)
	require.True(t, exprA == gotExpr)

	require.NoError(t, exprA.AddChild(exprB))
	gotExpr, ok = NodeUnwrap[*ExprNode](exprA)
	require.True(t, exprB == gotExpr)

	litA := &LiteralNode{}
	var gotLit *LiteralNode
	require.NoError(t, exprB.AddChild(litA))

	gotLit, ok = NodeUnwrap[*LiteralNode](exprA)
	require.True(t, ok)
	require.True(t, litA == gotLit)

	litB := &LiteralNode{}
	require.NoError(t, exprB.AddChild(litB))
	gotLit, ok = NodeUnwrap[*LiteralNode](exprA)
	require.False(t, ok, "should fail because exprB has multiple children")
	require.Nil(t, gotLit)
}
