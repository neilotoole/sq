package sqlbuilder

import (
	"github.com/neilotoole/sq/libsq/ast"
)

// Renderer renders ast elements into SQL string fragments.
type Renderer struct {
	// FromTable renders a FROM table fragment.
	FromTable func(bc *BuildContext, r *Renderer, tblSel *ast.TblSelectorNode) (string, error)

	// SelectCols renders a column names/expression fragment.
	// It shouldn't render the actual SELECT keyword. Example:
	//
	//   "first_name", "last name" AS given_name
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
	}
}
