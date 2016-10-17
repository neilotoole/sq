package ast

import (
	"testing"

	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/stretchr/testify/assert"
)

func TestWalker(t *testing.T) {

	// `@mydb1 | .user, .address | join(.uid == .uid) | .uid, .username, .country`
	p := getParser(FixtJoinQuery1)
	query := p.Query()
	ast, err := NewBuilder(drvr.NewSourceSet()).Build(query)

	assert.Nil(t, err)
	assert.NotNil(t, ast)

	walker := NewWalker(ast)
	count := 0

	visitor := func(w *Walker, node Node) error {
		count++
		return w.visitChildren(node)
	}

	walker.AddVisitor(TypeFnJoin, visitor)
	err = walker.Walk()
	assert.Nil(t, err)
	assert.Equal(t, 1, count)

	// test multiple visitors on the same node type
	walker = NewWalker(ast)
	countA := 0
	visitorA := func(w *Walker, node Node) error {
		countA++
		return w.visitChildren(node)
	}
	countB := 0
	visitorB := func(w *Walker, node Node) error {
		countB++
		return w.visitChildren(node)
	}

	walker.AddVisitor(TypeTableSelector, visitorA)
	walker.AddVisitor(TypeColSelector, visitorB)
	err = walker.Walk()
	assert.Nil(t, err)
	assert.Equal(t, 2, countA)
	assert.Equal(t, 5, countB)

}
