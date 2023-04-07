package sqlbuilder

import (
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func doOperator(bc *BuildContext, _ *Renderer, op *ast.OperatorNode) (string, error) {
	if op == nil {
		return "", nil
	}

	val, ok := bc.Dialect.Ops[op.Text()]
	if !ok {
		return "", errz.Errorf("invalid operator: %s", op.Text())
	}

	rhs := ast.NodeNextSibling(op)
	if lit, ok := rhs.(*ast.LiteralNode); ok && lit.Text() == "null" {
		switch op.Text() {
		case "==":
			val = "IS"
		case "!=":
			val = "IS NOT"
		default:
			return "", errz.Errorf("invalid operator for null")
		}
	}

	return val, nil
}
