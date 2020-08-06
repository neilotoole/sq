package ast

import (
	"fmt"

	"github.com/antlr/antlr4/runtime/Go/antlr"

	"github.com/neilotoole/sq/libsq/slq"
)

const msgNodeNoAddChild = "%T cannot add children: failed to add %T"
const msgNodeNoAddChildren = "%T cannot add children: failed to add %d children"

var _ Node = (*Selector)(nil)

// Selector is a selector such as ".my_table" or ".my_col". The
// generic selector will typically be replaced with a more specific
// selector node such as TblSelector or ColSelector.
type Selector struct {
	baseNode
}

func (s *Selector) String() string {
	return nodeString(s)
}

func (s *Selector) SelValue() string {
	return s.ctx.GetText()[1:]
}

var _ Node = (*TblSelector)(nil)

// TblSelector is a selector for a table, such as ".my_table"
// or "@my_src.my_table".
type TblSelector struct {
	Selector
	DSName  string
	TblName string
}

func newTblSelector(seg *Segment, tblName string, ctx antlr.ParseTree) *TblSelector {
	tbl := &TblSelector{}
	tbl.parent = seg
	tbl.ctx = ctx
	tbl.TblName = tblName
	return tbl
}

// Selectable implements the Selectable marker interface.
func (s *TblSelector) Selectable() {
	// no-op
}
func (s *TblSelector) SelValue() string {
	return s.TblName
}

func (s *TblSelector) String() string {
	text := nodeString(s)
	text += fmt.Sprintf(" | table: %q | datasource: %q", s.SelValue(), s.DSName)
	return text
}

var _ Node = (*ColSelector)(nil)
var _ ColExpr = (*ColSelector)(nil)

// ColSelector models a column selector such as ".user_id".
type ColSelector struct {
	Selector
}

func newColSelector(parent Node, ctx antlr.ParseTree) *ColSelector {
	col := &ColSelector{}
	col.parent = parent
	col.ctx = ctx
	return col
}

// ColExpr returns the column name.
func (s *ColSelector) ColExpr() (string, error) {
	// Drop the leading dot, e.g. ".user" -> "user"
	return s.Text()[1:], nil
}

func (s *ColSelector) IsColName() bool {
	return true
}

func (s *ColSelector) String() string {
	return nodeString(s)
}

var _ Node = (*Cmpr)(nil)

// Cmpr models a comparison.
type Cmpr struct {
	baseNode
}

func (c *Cmpr) String() string {
	return nodeString(c)
}

func newCmnr(parent Node, ctx slq.ICmprContext) *Cmpr {
	leaf := ctx.GetChild(0).(*antlr.TerminalNodeImpl)
	cmpr := &Cmpr{}
	cmpr.ctx = leaf
	cmpr.parent = parent
	return cmpr
}

// Datasource models a source such as "@sakila_sl3".
type Datasource struct {
	baseNode
}

func (d *Datasource) String() string {
	return nodeString(d)
}
