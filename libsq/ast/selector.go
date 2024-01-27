package ast

import (
	"fmt"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
)

// VisitSelectorElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSelectorElement(ctx *slq.SelectorElementContext) any {
	node, err := newSelectorNode(v.cur, ctx.Selector())
	if err != nil {
		return err
	}

	if aliasCtx := ctx.Alias(); aliasCtx != nil {
		if aliasCtx.ID() != nil {
			node.alias = aliasCtx.ID().GetText()
		}
		if aliasCtx.STRING() != nil {
			node.alias = stringz.StripDoubleQuote(aliasCtx.STRING().GetText())
		}
	}

	if err := v.cur.AddChild(node); err != nil {
		return err
	}

	// No need to descend to the children, because we've already dealt
	// with them in this function.
	return nil
}

// VisitSelector implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSelector(ctx *slq.SelectorContext) any {
	node, err := newSelectorNode(v.cur, ctx)
	if err != nil {
		return err
	}

	return v.cur.AddChild(node)
}

const (
	msgNodeNoAddChild    = "%T cannot add children: failed to add %T"
	msgNodeNoAddChildren = "%T cannot add children: failed to add %d children"
)

func newSelectorNode(parent Node, ctx slq.ISelectorContext) (*SelectorNode, error) {
	selNode := &SelectorNode{}
	selNode.parent = parent
	selNode.ctx = ctx
	selNode.text = ctx.GetText()

	var err error
	names := ctx.AllNAME()
	switch len(names) {
	default:
		return nil, errorf("expected 1 or 2 name parts in selector (e.g. '.table.column') but got %d parts: %s",
			len(names), ctx.GetText())
	case 1:
		if selNode.name0, err = extractSelVal(names[0]); err != nil {
			return nil, err
		}
	case 2:
		if selNode.name0, err = extractSelVal(names[0]); err != nil {
			return nil, err
		}
		if selNode.name1, err = extractSelVal(names[1]); err != nil {
			return nil, err
		}
	}

	return selNode, nil
}

var (
	_ Node     = (*SelectorNode)(nil)
	_ Selector = (*SelectorNode)(nil)
)

// SelectorNode is a selector such as ".my_table" or ".my_col". The
// generic selector will typically be replaced with a more specific
// selector node such as TblSelectorNode or ColSelectorNode.
type SelectorNode struct {

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
	baseNode
}

// selector implements the ast.selector marker interface.
func (s *SelectorNode) selector() {
}

// Strings returns a log/debug-friendly representation.
func (s *SelectorNode) String() string {
	return nodeString(s)
}

// SelValue returns the selector value.
// See extractSelValue.
func (s *SelectorNode) SelValue() (string, error) {
	return extractSelVal(s.ctx)
}

var _ Node = (*TblSelectorNode)(nil)

// TblSelectorNode is a selector for a table, such as ".my_table"
// or "@my_src.my_table".
type TblSelectorNode struct {
	tbl tablefq.T

	handle string
	SelectorNode
}

// newTblSelector creates a new TblSelectorNode from ctx.
func newTblSelector(selNode *SelectorNode) (*TblSelectorNode, error) { //nolint:unparam
	n := &TblSelectorNode{
		SelectorNode: *selNode,
		tbl:          tablefq.From(selNode.name0),
	}

	return n, nil
}

// SetTable sets the table value.
func (n *TblSelectorNode) SetTable(tbl tablefq.T) {
	n.tbl = tbl
}

// Table returns the table name. This is the raw value without punctuation.
func (n *TblSelectorNode) Table() tablefq.T {
	return n.tbl
}

// SyncTblNameAlias sets the table name to the alias value,
// if the alias is non-empty, and then sets the alias to empty.
func (n *TblSelectorNode) SyncTblNameAlias() {
	if n.alias != "" {
		n.tbl = tablefq.From(n.alias)
		n.alias = ""
	}
}

// TblAliasOrName returns the table alias if set; if not, it
// returns the table name.
func (n *TblSelectorNode) TblAliasOrName() tablefq.T {
	if n.alias != "" {
		t := n.tbl
		t.Table = n.alias
		return t
	}

	return n.tbl
}

// Alias returns the node's alias, or empty string.
func (n *TblSelectorNode) Alias() string {
	return n.alias
}

// Handle returns the handle, which may be empty.
func (n *TblSelectorNode) Handle() string {
	return n.handle
}

// SetHandle sets the handle.
func (n *TblSelectorNode) SetHandle(h string) {
	n.handle = h
}

// SelValue returns the table name.
// TODO: Can we get rid of this method SelValue?
func (n *TblSelectorNode) SelValue() (string, error) {
	return n.tbl.Table, nil
}

// String returns a log/debug-friendly representation.
func (n *TblSelectorNode) String() string {
	text := nodeString(n)
	selVal, err := n.SelValue()
	if err != nil {
		selVal = "error: " + err.Error()
	}
	text += fmt.Sprintf(" | table: {%s} | datasource: {%s}", selVal, n.Handle())
	return text
}

var (
	_ Node         = (*TblColSelectorNode)(nil)
	_ ResultColumn = (*TblColSelectorNode)(nil)
	_ Selector     = (*TblColSelectorNode)(nil)
)

// TblColSelectorNode models the TABLE.COLUMN selector, e.g. actor.first_name.
type TblColSelectorNode struct {
	*SelectorNode
	tblName string
	colName string
}

// resultColumn implements ast.ResultColumn.
func (n *TblColSelectorNode) resultColumn() {
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
	_ Selector     = (*ColSelectorNode)(nil)
)

// ColSelectorNode models a column selector such as ".first_name".
type ColSelectorNode struct {
	*SelectorNode

	colName string
}

// resultColumn implements ast.ResultColumn.
func (n *ColSelectorNode) resultColumn() {
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

// extractSelVal extracts the value of the selector. The function takes
// a selector node type as input, e.g. ast.SelectorNode.
// Example inputs:
//
//	.actor 					-->		actor
//	.first_name 		--> 	first_name
//	."first name" 	--> 	first name
//
// The function will remove the leading period, and any quotes around the name.
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
