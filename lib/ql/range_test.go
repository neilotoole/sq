package ql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// []      select all rows (no range)
// [1]     select row[1]
// [10:15] select rows 10 thru 15
// [0:15]  select rows 0 thru 15
// [:15]   same as above (0 thru 15)
// [10:]   select all rows from 10 onwards

func TestRowRange1(t *testing.T) {

	ast := _getAST(t, fixtRowRange1)
	assert.Equal(t, 0, NewInspector(ast).countNodes(TypeRowRange))

}

func TestRowRange2(t *testing.T) {

	ast := _getAST(t, fixtRowRange2)
	ins := NewInspector(ast)
	assert.Equal(t, 1, ins.countNodes(TypeRowRange))
	nodes := ins.findNodes(TypeRowRange)
	assert.Equal(t, 1, len(nodes))
	rr := nodes[0].(*RowRange)
	assert.Equal(t, 2, rr.offset)
	assert.Equal(t, 1, rr.limit)
}

func TestRowRange3(t *testing.T) {

	ast := _getAST(t, fixtRowRange3)
	ins := NewInspector(ast)
	rr := ins.findNodes(TypeRowRange)[0].(*RowRange)
	assert.Equal(t, 1, rr.offset)
	assert.Equal(t, 2, rr.limit)
}

func TestRowRange4(t *testing.T) {

	ast := _getAST(t, fixtRowRange4)
	ins := NewInspector(ast)
	rr := ins.findNodes(TypeRowRange)[0].(*RowRange)
	assert.Equal(t, 0, rr.offset)
	assert.Equal(t, 3, rr.limit)
}

func TestRowRange5(t *testing.T) {

	ast := _getAST(t, fixtRowRange5)
	ins := NewInspector(ast)
	rr := ins.findNodes(TypeRowRange)[0].(*RowRange)
	assert.Equal(t, 0, rr.offset)
	assert.Equal(t, 3, rr.limit)
}
func TestRowRange6(t *testing.T) {

	ast := _getAST(t, fixtRowRange6)
	ins := NewInspector(ast)
	rr := ins.findNodes(TypeRowRange)[0].(*RowRange)
	assert.Equal(t, 2, rr.offset)
	assert.Equal(t, -1, rr.limit)
}

func _getAST(t *testing.T, query string) *AST {
	p := getParser(query)
	q := p.Query()
	ast, err := BuildAST(q)
	assert.Nil(t, err)
	assert.NotNil(t, ast)
	return ast
}
