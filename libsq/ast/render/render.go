// Package render provides the mechanism for rendering ast into SQL.
package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver/dialect"
)

// Context contains context for rendering a query.
type Context struct {
	// Renderer holds the rendering functions.
	Renderer *Renderer

	// The args map contains predefined variables that are
	// substituted into the query. It may be empty or nil.
	Args map[string]string

	// Fragments is the set of fragments that are rendered into
	// a SQL query. It may not be initialized until late in
	// the day.
	Fragments *Fragments

	// Dialect is the driver dialect.
	Dialect dialect.Dialect
}

// Renderer is a set of functions for rendering ast elements into SQL.
// Use NewDefaultRenderer to get a new instance. Each function can be
// swapped with a custom implementation for a SQL dialect.
type Renderer struct {
	// FromTable renders a FROM table fragment.
	FromTable func(rc *Context, tblSel *ast.TblSelectorNode) (string, error)

	// SelectCols renders a column names/expression fragment.
	// It shouldn't render the actual SELECT keyword. Example return value:
	//
	//   "first_name" AS "given_name", "last name" AS "family_name"
	SelectCols func(rc *Context, cols []ast.ResultColumn) (string, error)

	// Range renders a row range fragment.
	Range func(rc *Context, rr *ast.RowRangeNode) (string, error)

	// OrderBy renders the ORDER BY fragment.
	OrderBy func(rc *Context, ob *ast.OrderByNode) (string, error)

	// GroupBy renders the GROUP BY fragment.
	GroupBy func(rc *Context, gb *ast.GroupByNode) (string, error)

	// Having renders the HAVING fragment.
	Having func(rc *Context, having *ast.HavingNode) (string, error)

	// Join renders a join fragment.
	Join func(rc *Context, leftTbl *ast.TblSelectorNode, joins []*ast.JoinNode) (string, error)

	// Function renders a function fragment.
	Function func(rc *Context, fn *ast.FuncNode) (string, error)

	// FunctionNames is a map of SLQ function name to SQL function name.
	// It can be used by the Renderer.Function impl. Note that FunctionOverrides
	// has precedence over FunctionNames.
	FunctionNames map[string]string

	// FunctionOverrides is a map of SLQ function name to a custom
	// function to render that function. It can be used by the Renderer.Function
	// imp. FunctionOverrides has precedence over FunctionNames.
	FunctionOverrides map[string]func(rc *Context, fn *ast.FuncNode) (string, error)

	// Literal renders a literal fragment.
	Literal func(rc *Context, lit *ast.LiteralNode) (string, error)

	// Where renders a WHERE fragment.
	Where func(rc *Context, where *ast.WhereNode) (string, error)

	// Expr renders an expression fragment.
	Expr func(rc *Context, expr *ast.ExprNode) (string, error)

	// Operator renders an operator fragment.
	Operator func(rc *Context, op *ast.OperatorNode) (string, error)

	// Distinct renders the DISTINCT fragment. Returns an
	// empty string if n is nil.
	Distinct func(rc *Context, n *ast.UniqueNode) (string, error)

	// Render renders f into a SQL query.
	Render func(rc *Context, f *Fragments) (string, error)

	// PreRender is a set of hooks that are called before Render. It is a final
	// opportunity to customize f before rendering. It is nil by default.
	PreRender []func(rc *Context, f *Fragments) error
}

// NewDefaultRenderer returns a Renderer that works for most SQL dialects.
// Driver implementations can override specific rendering functions
// as needed.
func NewDefaultRenderer() *Renderer {
	return &Renderer{
		FromTable:  doFromTable,
		SelectCols: doSelectCols,
		Range:      doRange,
		OrderBy:    doOrderBy,
		GroupBy:    doGroupBy,
		Having:     doHaving,
		Join:       doJoin,
		Function:   doFunction,
		FunctionOverrides: map[string]func(rc *Context, fn *ast.FuncNode) (string, error){
			ast.FuncNameRowNum: doFuncRowNum,
		},
		FunctionNames: map[string]string{},
		Literal:       doLiteral,
		Where:         doWhere,
		Expr:          doExpr,
		Operator:      doOperator,
		Distinct:      doDistinct,
		Render:        doRender,
	}
}

// Fragments holds the fragments of a SQL query.
// It is passed to Renderer.PreRender and Renderer.Render.
type Fragments struct {
	Distinct string
	Columns  string
	From     string
	Where    string
	GroupBy  string
	Having   string
	OrderBy  string
	Range    string
	// PreExecStmts are statements that are executed before the query.
	// These can be used for edge-case behavior, such as setting up
	// variables in the session.
	//
	// See also: Fragments.PostExecStmts.
	PreExecStmts []string

	// PostExecStmts are statements that are executed after the query.
	//
	// See also: Fragments.PreExecStmts.
	PostExecStmts []string
}

// doRender renders the supplied fragments into a SQL query.
func doRender(_ *Context, f *Fragments) (string, error) {
	sb := strings.Builder{}

	sb.WriteString("SELECT")

	if f.Distinct != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Distinct)
	}

	sb.WriteRune(sp)
	sb.WriteString(f.Columns)

	if f.From != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.From)
	}

	if f.Where != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Where)
	}

	if f.GroupBy != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.GroupBy)
	}

	if f.Having != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Having)
	}

	if f.OrderBy != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.OrderBy)
	}

	if f.Range != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Range)
	}

	return sb.String(), nil
}

const (
	singleQuote = '\''
	sp          = ' '
)

// renderSelectorNode renders a selector such as ".actor.first_name"
// or ".last_name".
func renderSelectorNode(d dialect.Dialect, node ast.Node) (string, error) {
	switch node := node.(type) {
	case *ast.ColSelectorNode:
		return d.Enquote(node.ColName()), nil
	case *ast.TblColSelectorNode:
		return d.Enquote(node.TblName()) + "." + d.Enquote(node.ColName()), nil
	case *ast.TblSelectorNode:
		return node.Table().Render(d.Enquote), nil
	default:
		return "", errz.Errorf(
			"expected selector node type, but got %T: %s",
			node,
			node.Text(),
		)
	}
}

// AppendSQL is a convenience function for building the SQL string.
// The main purpose is to ensure that there's always a consistent amount
// of whitespace. Thus, if existing has a space suffix and add has a
// space prefix, the returned string will only have one space. If add
// is the empty string or just whitespace, this function simply
// returns existing.
func AppendSQL(existing, add string) string {
	add = strings.TrimSpace(add)
	if add == "" {
		return existing
	}

	existing = strings.TrimSpace(existing)
	return existing + " " + add
}

// unquoteLiteral returns true if s is a double-quoted string, and also returns
// the value with the quotes stripped. An error is returned if the string
// is malformed.
func unquoteLiteral(s string) (val string, ok bool, err error) {
	hasPrefix := strings.HasPrefix(s, `"`)
	hasSuffix := strings.HasSuffix(s, `"`)

	if hasPrefix && hasSuffix {
		val = strings.TrimPrefix(s, `"`)
		val = strings.TrimSuffix(val, `"`)
		return val, true, nil
	}

	if hasPrefix != hasSuffix {
		return "", false, errz.Errorf("malformed literal: %s", s)
	}

	return s, false, nil
}

// FuncOverrideString returns a function that always returns s.
func FuncOverrideString(s string) func(*Context, *ast.FuncNode) (string, error) {
	return func(_ *Context, _ *ast.FuncNode) (string, error) {
		return s, nil
	}
}
