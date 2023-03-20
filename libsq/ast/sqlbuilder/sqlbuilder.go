// Package sqlbuilder contains functionality for building SQL from
// the AST.
package sqlbuilder

import (
	"github.com/neilotoole/sq/libsq/ast"
)

// FragmentBuilder renders driver-specific SQL fragments.
type FragmentBuilder interface {
	// FromTable renders a FROM table fragment.
	FromTable(tblSel *ast.TblSelectorNode) (string, error)

	// SelectCols renders a column names/expression fragment.
	SelectCols(cols []ast.ResultColumn) (string, error)

	// SelectAll renders a SELECT * fragment.
	SelectAll(tblSel *ast.TblSelectorNode) (string, error)

	// Range renders a row range fragment.
	Range(rr *ast.RowRange) (string, error)

	// Join renders a join fragment.
	Join(fnJoin *ast.JoinNode) (string, error)

	// Function renders a function fragment.
	Function(fn *ast.Func) (string, error)

	// Where renders a WHERE fragment.
	Where(where *ast.Where) (string, error)

	// Expr renders an expression fragment.
	Expr(expr *ast.Expr) (string, error)

	// Operator renders an operator fragment.
	Operator(op *ast.Operator) (string, error)
}

// QueryBuilder provides an abstraction for building a SQL query.
type QueryBuilder interface {
	// SetSelect sets the columns to select.
	SetSelect(cols string)

	// SetFrom sets the FROM clause.
	SetFrom(from string)

	// SetWhere sets the WHERE clause.
	SetWhere(where string)

	// SetRange sets the range clause.
	SetRange(rng string)

	// SQL renders the SQL query.
	SQL() (string, error)
}
