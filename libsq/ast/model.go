package ast

// Selectable is a marker interface to indicate that the entity can be selected from,
// e.g. a table or a join.
type Selectable interface {
	// From returns a SQL FROM clause.
	//From() func()(*fromClause, error)
	Selectable()
}

// ColExpr indicates a column selection expression, e.g. a column name, or
// context-appropriate function (e.g. "COUNT(*)")
type ColExpr interface {
	ColExpr() (string, error)
	String() string
}
