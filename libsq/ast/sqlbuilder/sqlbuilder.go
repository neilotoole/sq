// Package sqlbuilder contains functionality for building SQL from
// the ast.
package sqlbuilder

import (
	"github.com/neilotoole/sq/libsq/ast"
)

// FragmentBuilder renders driver-specific SQL fragments.
type FragmentBuilder interface {
	// FromTable renders a FROM table fragment.
	FromTable(tblSel *ast.TblSelectorNode) (string, error)

	// SelectCols renders a column names/expression fragment.
	// It shouldn't render the actual SELECT keyword. Example:
	//
	//   "first_name", "last name" AS given_name
	SelectCols(cols []ast.ResultColumn) (string, error)

	// Range renders a row range fragment.
	Range(rr *ast.RowRangeNode) (string, error)

	// OrderBy renders the ORDER BY fragment.
	OrderBy(ob *ast.OrderByNode) (string, error)

	// GroupBy renders the GROUP BY fragment.
	GroupBy(gb *ast.GroupByNode) (string, error)

	// Join renders a join fragment.
	Join(fnJoin *ast.JoinNode) (string, error)

	// Function renders a function fragment.
	Function(fn *ast.FuncNode) (string, error)

	// Where renders a WHERE fragment.
	Where(where *ast.WhereNode) (string, error)

	// Expr renders an expression fragment.
	Expr(expr *ast.ExprNode) (string, error)

	// Operator renders an operator fragment.
	Operator(op *ast.OperatorNode) (string, error)

	// Distinct renders the DISTINCT fragment. Returns an
	// empty string if n is nil.
	Distinct(n *ast.UniqueNode) (string, error)
}

// QueryBuilder provides an abstraction for building a SQL query.
type QueryBuilder interface {
	// SetColumns sets the columns to select.
	SetColumns(cols string)

	// SetFrom sets the FROM clause.
	SetFrom(from string)

	// SetWhere sets the WHERE clause.
	SetWhere(where string)

	// SetRange sets the LIMIT ... OFFSET clause.
	SetRange(rng string)

	// SetOrderBy sets the ORDER BY clause.
	SetOrderBy(ob string)

	// SetGroupBy sets the GROUP BY clause.
	SetGroupBy(gb string)

	// SetDistinct sets the DISTINCT clause.
	SetDistinct(d string)

	// Render renders the SQL query.
	Render() (string, error)
}
