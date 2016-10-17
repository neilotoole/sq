package datatype

import (
	"fmt"
)

// Type models a generic data type, which ultimately maps to some more specific
// implementation data type, such as a SQL VARCHAR or JSON boolean.
type Type int

func (d Type) String() string {

	switch d {
	case Text:
		return "text"
	case Int:
		return "int"
	case Float:
		return "float"
	case Decimal:
		return "decimal"
	case Bool:
		return "bool"
	case DateTime:
		return "datetime"
	case Bytes:
		return "bytes"
	case Null:
		return "null"
	}

	panic(fmt.Sprintf("unknown data type %q", d))
}

const (
	_         = iota
	Text Type = 1 << iota
	Int
	Float
	Decimal
	Bool
	DateTime
	Bytes
	Null
)
