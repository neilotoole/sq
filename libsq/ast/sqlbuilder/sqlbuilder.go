package sqlbuilder

import (
	"strings"

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

// Renderer renders ast elements into SQL.
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

	// PreRender is a hook that is called before Render. It allows a final
	// chance to customize f before rendering. It is nil by default.
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
