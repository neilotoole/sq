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

var _ Node = (*SelectorNode)(nil)

// SelectorNode is a selector such as ".my_table" or ".my_col". The
// generic selector will typically be replaced with a more specific
// selector node such as TblSelectorNode or ColSelectorNode.
type SelectorNode struct {
	baseNode

	// alias is the (optional) alias part. For example, given ".first_name:given_name",
	// the alias value is "given_name". May be empty.
	alias string

	// name0 is the first name part.
	//  - .actor -> name0 = .actor
	//  - .first_name -> name0 = .first_name
	//  - .actor.first_name -> name0 = .actor
	name0 string

	// name1 is the optional second name part.
	// - .actor --> name1 = EMPTY
	// - .actor.first_name -> name1 = first_name
	name1 string
}

func (s *SelectorNode) String() string {
	return nodeString(s)
}

// SelValue returns the selector value.
// See extractSelValue.
func (s *SelectorNode) SelValue() (string, error) {
	return extractSelVal(s.ctx)
}

var (
	_ Node       = (*TblSelectorNode)(nil)
	_ Selectable = (*TblSelectorNode)(nil)
)

// TblSelectorNode is a selector for a table, such as ".my_table"
// or "@my_src.my_table".
type TblSelectorNode struct {
	SelectorNode

	handle  string
	tblName string
}

// newTblSelector creates a new TblSelectorNode from ctx.
func newTblSelector(selNode *SelectorNode) (*TblSelectorNode, error) { //nolint:unparam
	n := &TblSelectorNode{
		SelectorNode: *selNode,
		tblName:      selNode.name0,
	}

	return n, nil
}

// TblName returns the table name. This is the raw value without punctuation.
func (s *TblSelectorNode) TblName() string {
	return s.tblName
}

// Handle returns the handle, which may be empty.
func (s *TblSelectorNode) Handle() string {
	return s.handle
}

// Selectable implements the Selectable marker interface.
func (s *TblSelectorNode) Selectable() {
	// no-op
}

// SelValue returns the table name.
// TODO: Can we get rid of this method SelValue?
func (s *TblSelectorNode) SelValue() (string, error) {
	return s.TblName(), nil
}

// String returns a log/debug-friendly representation.
func (s *TblSelectorNode) String() string {
	text := nodeString(s)
	selVal, err := s.SelValue()
	if err != nil {
		selVal = "error: " + err.Error()
	}
	text += fmt.Sprintf(" | table: %q | datasource: %q", selVal, s.Handle())
	return text
}

var (
	_ Node         = (*TblColSelectorNode)(nil)
	_ ResultColumn = (*TblColSelectorNode)(nil)
)

// TblColSelectorNode models the TABLE.COLUMN selector, e.g. actor.first_name.
type TblColSelectorNode struct {
	*SelectorNode
	tblName string
	colName string
}

// IsColumn implements ResultColumn.
func (n *TblColSelectorNode) IsColumn() bool {
	return true
}

// Text implements ResultColumn.
func (n *TblColSelectorNode) Text() string {
	return n.ctx.GetText()
}

func newTblColSelectorNode(selNode *SelectorNode) (*TblColSelectorNode, error) {
	n := &TblColSelectorNode{
		SelectorNode: selNode,
		tblName:      selNode.name0,
		colName:      selNode.name1,
	}

	if n.tblName == "" {
		return nil, errorf("cannot create %T: table name is empty: %s", n, n.Text())
	}

	if n.colName == "" {
		return nil, errorf("cannot create %T: column name is empty: %s", n, n.Text())
	}

	return n, nil
}

// TblName returns the table name, e.g. actor.
func (n *TblColSelectorNode) TblName() string {
	return n.tblName
}

// ColName returns the column name, e.g. first_name.
func (n *TblColSelectorNode) ColName() string {
	return n.colName
}

// String returns a log/debug-friendly representation.
func (n *TblColSelectorNode) String() string {
	return fmt.Sprintf("%T: %s", n, n.ctx.GetText())
}

// Alias returns the column alias, which may be empty.
// For example, given the selector ".first_name:given_name",
// the alias is "given_name".
func (n *TblColSelectorNode) Alias() string {
	return n.alias
}

var (
	_ Node         = (*ColSelectorNode)(nil)
	_ ResultColumn = (*ColSelectorNode)(nil)
)

// ColSelectorNode models a column selector such as ".first_name".
type ColSelectorNode struct {
	*SelectorNode

	colName string
}

// newColSelectorNode returns a ColSelectorNode constructed from ctx.
func newColSelectorNode(selNode *SelectorNode) (*ColSelectorNode, error) { //nolint:unparam
	n := &ColSelectorNode{SelectorNode: selNode}
	n.colName = selNode.name0
	return n, nil
}

// Text implements ResultColumn.
func (n *ColSelectorNode) Text() string {
	return n.ctx.GetText()
}

// IsColumn always returns true.
func (n *ColSelectorNode) IsColumn() bool {
	return true
}

// ColName returns the column name. Note the name is not escaped/quoted,
// thus it could contain whitespace, etc.
func (n *ColSelectorNode) ColName() string {
	return n.colName
}

// Alias returns the column alias, which may be empty.
// For example, given the selector ".first_name:given_name",
// the alias is "given_name".
func (n *ColSelectorNode) Alias() string {
	return n.alias
}

// String returns a log/debug-friendly representation.
func (n *ColSelectorNode) String() string {
	str := nodeString(n)
	if n.alias != "" {
		str += ":" + n.alias
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
	leaf, _ := ctx.GetChild(0).(*antlr.TerminalNodeImpl) // FIXME: return an error
	cmpr := &Cmpr{}
	cmpr.ctx = leaf
	cmpr.parent = parent
	return cmpr
}

// HandleNode models a source handle such as "@sakila_sl3".
type HandleNode struct {
	baseNode
}

func (d *HandleNode) String() string {
	return nodeString(d)
}

// extractSelVal extracts the value of the selector. The function takes
// a selector node type as input, e.g. ast.SelectorNode.
// Example inputs:
//
//   - .actor --> actor
//   - .first_name --> first_name
//   - ."first name" --> first name
//
// The function will remove the leading period, and quotes around the name.
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
