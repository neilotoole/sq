package ast

import (
	"fmt"

	"github.com/neilotoole/sq/libsq/slq"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

const emsgNodeNoAddChild = "%T cannot have children: failed to add %T"
const emsgNodeNoAddChildren = "%T cannot have children: failed to add %d children"

type Selector struct {
	BaseNode
}

func (s *Selector) String() string {
	return nodeString(s)
}

func (s *Selector) SelValue() string {
	return s.ctx.GetText()[1:]
}

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

func (s *TblSelector) Selectable() {
	// no-op
}
func (s *TblSelector) SelValue() string {
	return s.TblName
}

// From returns the table name.
func (ts *TblSelector) From() (string, error) {
	// Drop the leading dot, e.g. ".user" -> "user"
	return ts.Text()[1:], nil
}

func (s *TblSelector) String() string {
	text := nodeString(s)
	text = text + fmt.Sprintf(" | table: %q | datasource: %q", s.SelValue(), s.DSName)
	return text
}

// ColSelector models a column name, e.g. '.user_id'.
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

func (s *ColSelector) String() string {
	return nodeString(s)
}

type Cmpr struct {
	BaseNode
}

func (c *Cmpr) String() string {
	return nodeString(c)
}

func NewCmpr(parent Node, ctx slq.ICmprContext) *Cmpr {
	leaf := ctx.GetChild(0).(*antlr.TerminalNodeImpl)
	cmpr := &Cmpr{}
	cmpr.ctx = leaf
	cmpr.parent = parent
	return cmpr
}

type Datasource struct {
	BaseNode
}

func (d *Datasource) String() string {
	return nodeString(d)
}
