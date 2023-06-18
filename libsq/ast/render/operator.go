package render

import (
	"github.com/neilotoole/sq/libsq/ast"
)

func doOperator(rc *Context, op *ast.OperatorNode) (string, error) {
	if op == nil {
		return "", nil
	}

	text := op.Text()
	// Check if the dialect overrides the operator.
	val, ok := rc.Dialect.Ops[text]
	if !ok {
		val = text
	}

	rhs := ast.NodeNextSibling(op)
	if lit, ok := ast.NodeUnwrap[*ast.LiteralNode](rhs); ok && lit.Text() == "null" {
		switch op.Text() {
		case "==":
			val = "IS"
		case "!=":
			val = "IS NOT"
		}
	}

	// By default, just return the operator unchanged.
	if operatorHasSpace(val) {
		val = " " + val + " "
	}

	return val, nil
}

func operatorHasSpace(op string) bool {
	switch op {
	case "-", "+", "*", "/":
		return false
	}
	return true
}
