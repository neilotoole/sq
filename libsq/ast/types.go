package ast

import (
	"reflect"

	"github.com/neilotoole/sq-driver/hackery/database/sql"
)

type NodeType reflect.Type

//var TypeNode = NodeType(reflect.TypeOf((*Node)(nil)).Elem())
//var TypeBaseNode = NodeType(reflect.TypeOf((*BaseNode)(nil)).Elem())
//var TypeIR = NodeType(reflect.TypeOf((*IR)(nil)).Elem())
//var TypeDatasource = NodeType(reflect.TypeOf((*Datasource)(nil)).Elem())
//var TypeSegment = NodeType(reflect.TypeOf((*Segment)(nil)).Elem())
//var TypeFnJoin = NodeType(reflect.TypeOf((*FnJoin)(nil)))
//var TypeFnJoinExpr = NodeType(reflect.TypeOf((*FnJoinExpr)(nil)).Elem())
//var TypeSelector = NodeType(reflect.TypeOf((*Selector)(nil)).Elem())
//var TypeColSelector = NodeType(reflect.TypeOf((*ColSelector)(nil)).Elem())
//var TypeTableSelector = NodeType(reflect.TypeOf((*TableSelector)(nil)).Elem())
//var TypeRowRange = NodeType(reflect.TypeOf((*RowRange)(nil)).Elem())

// TODO: Consider renaming these to, e.g. TypeSegmentP or TypeSegmentPtr?
var TypeNode = NodeType(reflect.TypeOf((*Node)(nil)))
var TypeBaseNode = NodeType(reflect.TypeOf((*BaseNode)(nil)))
var TypeAST = NodeType(reflect.TypeOf((*AST)(nil)))
var TypeDatasource = NodeType(reflect.TypeOf((*Datasource)(nil)))
var TypeSegment = NodeType(reflect.TypeOf((*Segment)(nil)))
var TypeFnJoin = NodeType(reflect.TypeOf((*FnJoin)(nil)))
var TypeFnJoinExpr = NodeType(reflect.TypeOf((*FnJoinExpr)(nil)))
var TypeSelector = NodeType(reflect.TypeOf((*Selector)(nil)))
var TypeColSelector = NodeType(reflect.TypeOf((*ColSelector)(nil)))
var TypeTableSelector = NodeType(reflect.TypeOf((*TblSelector)(nil)))
var TypeRowRange = NodeType(reflect.TypeOf((*RowRange)(nil)))
var TypeNullString = NodeType(reflect.TypeOf((*sql.NullString)(nil)))
var TypeNullInt64 = NodeType(reflect.TypeOf((*sql.NullInt64)(nil)))
var TypeNullFloat64 = NodeType(reflect.TypeOf((*sql.NullFloat64)(nil)))
var TypeByteArray = NodeType(reflect.TypeOf((*[]byte)(nil)))

var TypeSelectable = NodeType(reflect.TypeOf((*Selectable)(nil)).Elem())
var TypeColExpr = NodeType(reflect.TypeOf((*ColExpr)(nil)).Elem())
