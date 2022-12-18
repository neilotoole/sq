package ast

import (
	"testing"

	"github.com/neilotoole/lg/testlg"
	"github.com/stretchr/testify/assert"
)

// []      select all rows (no range)
// [1]     select row[1]
// [10:15] select rows 10 thru 15
// [0:15]  select rows 0 thru 15
// [:15]   same as above (0 thru 15)
// [10:]   select all rows from 10 onwards

func TestRowRange1(t *testing.T) {
	log := testlg.New(t).Strict(true)

	ast := mustBuildAST(t, fixtRowRange1)
	assert.Equal(t, 0, NewInspector(log, ast).CountNodes(typeRowRange))
}

func TestRowRange2(t *testing.T) {
	log := testlg.New(t).Strict(true)

	ast := mustBuildAST(t, fixtRowRange2)
	ins := NewInspector(log, ast)
	assert.Equal(t, 1, ins.CountNodes(typeRowRange))
	nodes := ins.FindNodes(typeRowRange)
	assert.Equal(t, 1, len(nodes))
	rr, _ := nodes[0].(*RowRange)
	assert.Equal(t, 2, rr.Offset)
	assert.Equal(t, 1, rr.Limit)
}

func TestRowRange3(t *testing.T) {
	log := testlg.New(t).Strict(true)

	ast := mustBuildAST(t, fixtRowRange3)
	ins := NewInspector(log, ast)
	rr := ins.FindNodes(typeRowRange)[0].(*RowRange)
	assert.Equal(t, 1, rr.Offset)
	assert.Equal(t, 2, rr.Limit)
}

func TestRowRange4(t *testing.T) {
	log := testlg.New(t).Strict(true)

	ast := mustBuildAST(t, fixtRowRange4)
	ins := NewInspector(log, ast)
	rr, _ := ins.FindNodes(typeRowRange)[0].(*RowRange)
	assert.Equal(t, 0, rr.Offset)
	assert.Equal(t, 3, rr.Limit)
}

func TestRowRange5(t *testing.T) {
	log := testlg.New(t).Strict(true)
	ast := mustBuildAST(t, fixtRowRange5)
	ins := NewInspector(log, ast)
	rr := ins.FindNodes(typeRowRange)[0].(*RowRange)
	assert.Equal(t, 0, rr.Offset)
	assert.Equal(t, 3, rr.Limit)
}

func TestRowRange6(t *testing.T) {
	log := testlg.New(t).Strict(true)
	ast := mustBuildAST(t, fixtRowRange6)
	ins := NewInspector(log, ast)
	rr, _ := ins.FindNodes(typeRowRange)[0].(*RowRange)
	assert.Equal(t, 2, rr.Offset)
	assert.Equal(t, -1, rr.Limit)
}
