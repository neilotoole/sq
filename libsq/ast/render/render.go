package render

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/libsq/core/dialect"

	"github.com/neilotoole/sq/libsq/ast"
)

// BuildContext contains context for building a query.
type BuildContext struct {
	// Dialect is the driver dialect.
	Dialect dialect.Dialect

	// The args map contains predefined variables that are
	// substituted into the query. It may be empty or nil.
	Args map[string]string
}

// Renderer is a set of functions for rendering ast elements into SQL.
// Use NewDefaultRenderer to get a new instance. Each function can be
// swapped with a custom implementation for a SQL dialect.
type Renderer struct {
	// FromTable renders a FROM table fragment.
	FromTable func(bc *BuildContext, r *Renderer, tblSel *ast.TblSelectorNode) (string, error)

	// SelectCols renders a column names/expression fragment.
	// It shouldn't render the actual SELECT keyword. Example return value:
	//
	//   "first_name" AS "given_name", "last name" AS "family_name"
	SelectCols func(bc *BuildContext, r *Renderer, cols []ast.ResultColumn) (string, error)

	// Range renders a row range fragment.
	Range func(bc *BuildContext, r *Renderer, rr *ast.RowRangeNode) (string, error)

	// OrderBy renders the ORDER BY fragment.
	OrderBy func(bc *BuildContext, r *Renderer, ob *ast.OrderByNode) (string, error)

	// GroupBy renders the GROUP BY fragment.
	GroupBy func(bc *BuildContext, r *Renderer, gb *ast.GroupByNode) (string, error)

	// Join renders a join fragment.
	Join func(bc *BuildContext, r *Renderer, fnJoin *ast.JoinNode) (string, error)

	// Function renders a function fragment.
	Function func(bc *BuildContext, r *Renderer, fn *ast.FuncNode) (string, error)

	// Literal renders a literal fragment.
	Literal func(bc *BuildContext, r *Renderer, lit *ast.LiteralNode) (string, error)

	// Where renders a WHERE fragment.
	Where func(bc *BuildContext, r *Renderer, where *ast.WhereNode) (string, error)

	// Expr renders an expression fragment.
	Expr func(bc *BuildContext, r *Renderer, expr *ast.ExprNode) (string, error)

	// Operator renders an operator fragment.
	Operator func(bc *BuildContext, r *Renderer, op *ast.OperatorNode) (string, error)

	// Distinct renders the DISTINCT fragment. Returns an
	// empty string if n is nil.
	Distinct func(bc *BuildContext, r *Renderer, n *ast.UniqueNode) (string, error)

	// PreRender is a hook that is called before Render. It is a final
	// opportunity to customize f before rendering. It is nil by default.
	PreRender func(bc *BuildContext, r *Renderer, f *Fragments) error

	// Render renders f into a SQL query.
	Render func(bc *BuildContext, r *Renderer, f *Fragments) (string, error)
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
		Join:       doJoin,
		Function:   doFunction,
		Literal:    doLiteral,
		Where:      doWhere,
		Expr:       doExpr,
		Operator:   doOperator,
		Distinct:   doDistinct,
		Render:     doRender,
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
	OrderBy  string
	Range    string
}

// Render implements QueryBuilder.
func doRender(_ *BuildContext, _ *Renderer, f *Fragments) (string, error) {
	sb := strings.Builder{}

	sb.WriteString("SELECT")

	if f.Distinct != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Distinct)
	}

	sb.WriteRune(sp)
	sb.WriteString(f.Columns)
	sb.WriteRune(sp)
	sb.WriteString(f.From)

	if f.Where != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Where)
	}

	if f.OrderBy != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.OrderBy)
	}

	if f.GroupBy != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.GroupBy)
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
func renderSelectorNode(quote string, node ast.Node) (string, error) {
	// FIXME: switch to using enquote
	switch node := node.(type) {
	case *ast.ColSelectorNode:
		return fmt.Sprintf(
			"%s%s%s",
			quote,
			node.ColName(),
			quote,
		), nil
	case *ast.TblColSelectorNode:
		return fmt.Sprintf(
			"%s%s%s.%s%s%s",
			quote,
			node.TblName(),
			quote,
			quote,
			node.ColName(),
			quote,
		), nil
	case *ast.TblSelectorNode:
		return fmt.Sprintf(
			"%s%s%s",
			quote,
			node.TblName(),
			quote,
		), nil

	default:
		return "", errz.Errorf(
			"expected selector node type, but got %T: %s",
			node,
			node.Text(),
		)
	}
}

// sqlAppend is a convenience function for building the SQL string.
// The main purpose is to ensure that there's always a consistent amount
// of whitespace. Thus, if existing has a space suffix and add has a
// space prefix, the returned string will only have one space. If add
// is the empty string or just whitespace, this function simply
// returns existing.
func sqlAppend(existing, add string) string {
	add = strings.TrimSpace(add)
	if add == "" {
		return existing
	}

	existing = strings.TrimSpace(existing)
	return existing + " " + add
}

// quoteTableOrColSelector returns a quote table, col, or table/col
// selector for use in a SQL statement. For example:
//
//	.table     -->  "table"
//	.col       -->  "col"
//	.table.col -->  "table"."col"
//
// Thus, the selector must have exactly one or two periods.
//
// Deprecated: use renderSelectorNode.
func quoteTableOrColSelector(quote, selector string) (string, error) {
	if len(selector) < 2 || selector[0] != '.' {
		return "", errz.Errorf("invalid selector: %s", selector)
	}

	parts := strings.Split(selector[1:], ".")
	switch len(parts) {
	case 1:
		return quote + parts[0] + quote, nil
	case 2:
		return quote + parts[0] + quote + "." + quote + parts[1] + quote, nil
	default:
		return "", errz.Errorf("invalid selector: %s", selector)
	}
}

// escapeLiteral escapes the single quotes in s.
//
//	jessie's girl  -->  jessie''s girl
//
// See also: stringz.BacktickQuote.
func escapeLiteral(s string) string {
	sb := strings.Builder{}
	for _, r := range s {
		if r == singleQuote {
			_, _ = sb.WriteRune(singleQuote)
		}

		_, _ = sb.WriteRune(r)
	}

	return sb.String()
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
