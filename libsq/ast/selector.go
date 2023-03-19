package ast

import (
	"fmt"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

const (
	msgNodeNoAddChild    = "%T cannot add children: failed to add %T"
	msgNodeNoAddChildren = "%T cannot add children: failed to add %d children"
)

var _ Node = (*Selector)(nil)

// Selector is a selector such as ".my_table" or ".my_col". The
// generic selector will typically be replaced with a more specific
// selector node such as TblSelector or ColSelector.
type Selector struct {
	baseNode

	// alias is the (optional) alias part. For example, given ".first_name:given_name",
	// the alias value is "given_name". May be empty.
	alias string
}

func (s *Selector) String() string {
	return nodeString(s)
}

// SelValue returns the selector value.
// See extractSelValue.
func (s *Selector) SelValue() (string, error) {
	return extractSelVal(s.ctx)
}

var _ Node = (*TblSelector)(nil)

// TblSelector is a selector for a table, such as ".my_table"
// or "@my_src.my_table".
type TblSelector struct {
	Selector
	DSName  string
	TblName string
}

// newTblSelector creates a new TblSelector from ctx.
func newTblSelector(seg *Segment, ctx antlr.ParseTree) (*TblSelector, error) {
	tbl := &TblSelector{}
	tbl.parent = seg
	tbl.ctx = ctx

	var err error
	if tbl.TblName, err = extractSelVal(ctx); err != nil {
		return nil, err
	}
	return tbl, nil
}

// Selectable implements the Selectable marker interface.
func (s *TblSelector) Selectable() {
	// no-op
}

func (s *TblSelector) SelValue() (string, error) {
	return s.TblName, nil
}

// String returns a log/debug-friendly representation.
func (s *TblSelector) String() string {
	text := nodeString(s)
	selVal, err := s.SelValue()
	if err != nil {
		selVal = "error: " + err.Error()
	}
	text += fmt.Sprintf(" | table: %q | datasource: %q", selVal, s.DSName)
	return text
}

var (
	_ Node    = (*ColSelector)(nil)
	_ ColExpr = (*ColSelector)(nil)
)

// ColSelector models a column selector such as ".user_id".
type ColSelector struct {
	Selector

	alias string

	colName string
}

// newColSelector returns a ColSelector constructed from ctx.
func newColSelector(parent Node, ctx antlr.ParseTree, alias string) (*ColSelector, error) {
	col := &ColSelector{}
	col.parent = parent
	col.ctx = ctx
	col.alias = alias

	var err error
	if col.colName, err = extractSelVal(ctx); err != nil {
		return nil, err
	}
	return col, nil
}

// ColExpr returns the column name.
func (s *ColSelector) ColExpr() (string, error) {
	return extractSelVal(s.ctx)
}

// IsColName always returns true.
func (s *ColSelector) IsColName() bool {
	return true
}

// ColName returns the column name. Note the name is not escaped/quoted,
// thus it could contain whitespace, etc.
func (s *ColSelector) ColName() string {
	return s.colName
}

// Alias returns the column alias, which may be empty.
// For example, given the selector ".first_name:given_name",
// the alias is "given_name".
func (s *ColSelector) Alias() string {
	return s.alias
}

// String returns a log/debug-friendly representation.
func (s *ColSelector) String() string {
	str := nodeString(s)
	if s.alias != "" {
		str += ":" + s.alias
	}
	return str
}

var _ Node = (*Cmpr)(nil)

// Cmpr models a comparison, such as ".age == 42".
type Cmpr struct {
	baseNode
}

func (c *Cmpr) String() string {
	return nodeString(c)
}

func newCmpr(parent Node, ctx slq.ICmprContext) *Cmpr {
	leaf, _ := ctx.GetChild(0).(*antlr.TerminalNodeImpl)
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

// extractSelVal extracts the value of the selector.
// Example inputs:
//
//   - .actor --> actor
//   - .first_name --> first_name
//   - ."first name" --> first name
func extractSelVal(ctx antlr.ParseTree) (string, error) {
	if ctx == nil {
		return "", errorf("invalid selector: is nil")
	}
	original := ctx.GetText()
	if len(original) < 2 {
		return "", errorf("invalid selector expression: too short: %s", original)
	}

	if original[0] != '.' {
		return "", errorf("illegal selector expression: must start with period: %s", original)
	}

	// Trim the leading period, e.g. ".first_name" -> "first_name".
	wip := original[1:]

	// Remove quotes if applicable.
	if wip[0] == '"' {
		if wip[len(wip)-1] != '"' {
			return "", errorf("illegal selector expression: unmatched quotes on string: %s", original)
		}

		wip = strings.TrimPrefix(wip, `"`)
		wip = strings.TrimSuffix(wip, `"`)

		if len(wip) == 0 {
			return "", errorf("invalid selector expression: too short: %s", original)
		}
	}

	return wip, nil
}
