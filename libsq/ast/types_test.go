package ast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTypes_IsNode verifies that the package types implement appropriate interfaces etc.
func TestTypes_IsNode(t *testing.T) {

	// these "tests" will be caught statically by the compiler
	var node Node
	assert.Nil(t, node)

	node = &AST{}
	node = &Segment{}
	node = &Selector{}
	node = &TblSelector{}
	node = &ColSelector{}
	node = &Join{}
	node = &JoinConstraint{}
	node = &Cmpr{}
}
