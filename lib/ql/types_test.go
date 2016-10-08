package ql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNode_types verifies that the package types implement appropriate interfaces etc.
func TestTypes_IsNode(t *testing.T) {

	// these "tests" will be caught statically by the compiler
	var node Node
	assert.Nil(t, node)

	node = &AST{}
	node = &Segment{}
	node = &Selector{}
	node = &TblSelector{}
	node = &ColSelector{}
	node = &FnJoin{}
	node = &FnJoinExpr{}
	node = &Cmpr{}
}

//
//func TestTypes_Actual(t *testing.T) {
//
//	fmt.Printf("type: %s -- %v\n", TypeFnJoin, TypeFnJoin)
//
//	fn := &FnJoin{}
//
//	fnTyp := reflect.TypeOf(fn)
//	fmt.Printf("type2: %T\n", fn)
//	fmt.Printf("type: %s -- %v\n", fnTyp, fnTyp)
//}
