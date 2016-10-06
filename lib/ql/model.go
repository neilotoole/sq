package ql

//type Element interface {
//	//IsConsumed() bool
//	//Consumes() int
//	//Segment() *Segment
//	Value() string
//	Node() antlr.ParseTree
//	Segment() *Segment
//}

// Fromer is a marker interface to indicate that the entity can be selected from,
// e.g. a table or a join.
type Selectable interface {
	// From returns a SQL FROM clause.
	//From() func()(*fromClause, error)
	Selectable()
}

// ColExprer indicates a column selection expression, e.g. a column name, or
// context-appropriate function (e.g. "COUNT(*)")
type ColExpr interface {
	ColExpr() (string, error)
	String() string
}

type TableRefer interface {
	Alias() string
	TableRef() string
}

//type fromGenerator func() (ds string, fromClause string, params []interface{}, error)

type fromClause struct {
	ds     string
	tpl    string
	params []interface{}
}
