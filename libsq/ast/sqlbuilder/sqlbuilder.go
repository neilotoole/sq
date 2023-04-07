// Package sqlbuilder contains functionality for building SQL from
// the ast.
package sqlbuilder

import (
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/dialect"
)

// BuildContext contains context for building a query.
type BuildContext struct {
	// Dialect is the driver dialect.
	Dialect dialect.Dialect

	// The args map contains predefined variables that are
	// substituted into the query. It may be empty or nil.
	Args map[string]string

	// QuoteIdentFunc quotes an identifier. For example:
	//
	//  my_table -->  "my_table"
	QuoteIdentFunc func(ident string) string
}

// FragmentBuilder renders driver-specific SQL fragments.
//
//nolint:dupl
type FragmentBuilder interface {
	// FromTable renders a FROM table fragment.
	FromTable(bc *BuildContext, tblSel *ast.TblSelectorNode) (string, error)

	// SelectCols renders a column names/expression fragment.
	// It shouldn't render the actual SELECT keyword. Example:
	//
	//   "first_name", "last name" AS given_name
	SelectCols(bc *BuildContext, cols []ast.ResultColumn) (string, error)

	// Range renders a row range fragment.
	Range(bc *BuildContext, rr *ast.RowRangeNode) (string, error)

	// OrderBy renders the ORDER BY fragment.
	OrderBy(bc *BuildContext, ob *ast.OrderByNode) (string, error)

	// GroupBy renders the GROUP BY fragment.
	GroupBy(bc *BuildContext, gb *ast.GroupByNode) (string, error)

	// Join renders a join fragment.
	Join(bc *BuildContext, fnJoin *ast.JoinNode) (string, error)

	// Function renders a function fragment.
	Function(bc *BuildContext, fn *ast.FuncNode) (string, error)

	// Literal renders a literal fragment.
	Literal(bc *BuildContext, lit *ast.LiteralNode) (string, error)

	// Where renders a WHERE fragment.
	Where(bc *BuildContext, where *ast.WhereNode) (string, error)

	// Expr renders an expression fragment.
	Expr(bc *BuildContext, expr *ast.ExprNode) (string, error)

	// Operator renders an operator fragment.
	Operator(bc *BuildContext, op *ast.OperatorNode) (string, error)

	// Distinct renders the DISTINCT fragment. Returns an
	// empty string if n is nil.
	Distinct(bc *BuildContext, n *ast.UniqueNode) (string, error)
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
