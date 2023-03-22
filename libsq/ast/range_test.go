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

	ast := mustParse(t, fixtRowRange1)
	assert.Equal(t, 0, NewInspector(log, ast).CountNodes(typeRowRangeNode))
}

func TestRowRange2(t *testing.T) {
	log := testlg.New(t).Strict(true)

	ast := mustParse(t, fixtRowRange2)
	insp := NewInspector(log, ast)
	assert.Equal(t, 1, insp.CountNodes(typeRowRangeNode))
	nodes := insp.FindNodes(typeRowRangeNode)
	assert.Equal(t, 1, len(nodes))
	rr, _ := nodes[0].(*RowRange)
	assert.Equal(t, 2, rr.Offset)
	assert.Equal(t, 1, rr.Limit)
}

func TestRowRange3(t *testing.T) {
	log := testlg.New(t).Strict(true)

	ast := mustParse(t, fixtRowRange3)
	insp := NewInspector(log, ast)
	rr, _ := insp.FindNodes(typeRowRangeNode)[0].(*RowRange)
	assert.Equal(t, 1, rr.Offset)
	assert.Equal(t, 2, rr.Limit)
}

func TestRowRange4(t *testing.T) {
	log := testlg.New(t).Strict(true)

	ast := mustParse(t, fixtRowRange4)
	insp := NewInspector(log, ast)
	rr, _ := insp.FindNodes(typeRowRangeNode)[0].(*RowRange)
	assert.Equal(t, 0, rr.Offset)
	assert.Equal(t, 3, rr.Limit)
}

func TestRowRange5(t *testing.T) {
	log := testlg.New(t).Strict(true)
	ast := mustParse(t, fixtRowRange5)
	insp := NewInspector(log, ast)
	rr, _ := insp.FindNodes(typeRowRangeNode)[0].(*RowRange)
	assert.Equal(t, 0, rr.Offset)
	assert.Equal(t, 3, rr.Limit)
}

func TestRowRange6(t *testing.T) {
	log := testlg.New(t).Strict(true)
	ast := mustParse(t, fixtRowRange6)
	insp := NewInspector(log, ast)
	rr, _ := insp.FindNodes(typeRowRangeNode)[0].(*RowRange)
	assert.Equal(t, 2, rr.Offset)
	assert.Equal(t, -1, rr.Limit)
}
