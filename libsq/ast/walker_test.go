package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/neilotoole/lg"
	"github.com/neilotoole/lg/testlg"
)

func TestWalker(t *testing.T) {
	log := testlg.New(t).Strict(true)

	// `@mydb1 | .user, .address | join(.uid == .uid) | .uid, .username, .country`
	p := getSLQParser(fixtJoinQuery1)
	query := p.Query()
	ast, err := buildAst(log, query)

	assert.Nil(t, err)
	assert.NotNil(t, ast)

	walker := NewWalker(log, ast)
	count := 0

	visitor := func(log lg.Log, w *Walker, node Node) error {
		count++
		return w.visitChildren(node)
	}

	walker.AddVisitor(typeJoin, visitor)
	err = walker.Walk()
	assert.Nil(t, err)
	assert.Equal(t, 1, count)

	// test multiple visitors on the same node type
	walker = NewWalker(log, ast)
	countA := 0
	visitorA := func(log lg.Log, w *Walker, node Node) error {
		countA++
		return w.visitChildren(node)
	}
	countB := 0
	visitorB := func(log lg.Log, w *Walker, node Node) error {
		countB++
		return w.visitChildren(node)
	}

	walker.AddVisitor(typeTblSelector, visitorA)
	walker.AddVisitor(typeColSelector, visitorB)
	err = walker.Walk()
	assert.Nil(t, err)
	assert.Equal(t, 2, countA)
	assert.Equal(t, 5, countB)
}
