package sqlbuilder

import "github.com/neilotoole/sq/libsq/ast"

// Renderer renders ast elements into SQL string fragments.
//
//nolint:dupl
type Renderer struct {
	// FromTable renders a FROM table fragment.
	FromTable func(bc *BuildContext, tblSel *ast.TblSelectorNode) (string, error)

	// SelectCols renders a column names/expression fragment.
	// It shouldn't render the actual SELECT keyword. Example:
	//
	//   "first_name", "last name" AS given_name
	SelectCols func(bc *BuildContext, cols []ast.ResultColumn) (string, error)

	// Range renders a row range fragment.
	Range func(bc *BuildContext, rr *ast.RowRangeNode) (string, error)

	// OrderBy renders the ORDER BY fragment.
	OrderBy func(bc *BuildContext, ob *ast.OrderByNode) (string, error)

	// GroupBy renders the GROUP BY fragment.
	GroupBy func(bc *BuildContext, gb *ast.GroupByNode) (string, error)

	// Join renders a join fragment.
	Join func(bc *BuildContext, fnJoin *ast.JoinNode) (string, error)

	// Function renders a function fragment.
	Function func(bc *BuildContext, fn *ast.FuncNode) (string, error)

	// Literal renders a literal fragment.
	Literal func(bc *BuildContext, lit *ast.LiteralNode) (string, error)

	// Where renders a WHERE fragment.
	Where func(bc *BuildContext, where *ast.WhereNode) (string, error)

	// Expr renders an expression fragment.
	Expr func(bc *BuildContext, expr *ast.ExprNode) (string, error)

	// Operator renders an operator fragment.
	Operator func(bc *BuildContext, op *ast.OperatorNode) (string, error)

	// Distinct renders the DISTINCT fragment. Returns an
	// empty string if n is nil.
	Distinct func(bc *BuildContext, n *ast.UniqueNode) (string, error)
}
