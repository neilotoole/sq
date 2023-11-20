package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

func doExpr(rc *Context, expr *ast.ExprNode) (string, error) {
	if expr == nil {
		return "", nil
	}
	r := rc.Renderer

	var sb strings.Builder
	if expr.HasParens() {
		sb.WriteRune('(')
	}

	for _, child := range expr.Children() {
		switch child := child.(type) {
		case *ast.TblColSelectorNode, *ast.ColSelectorNode:
			val, err := renderSelectorNode(rc.Dialect, child)
			if err != nil {
				return "", err
			}
			sb.WriteString(val)
		case *ast.OperatorNode:
			val, err := r.Operator(rc, child)
			if err != nil {
				return "", err
			}

			sb.WriteString(val)
		case *ast.ArgNode:
			if rc.Args != nil {
				val, ok := rc.Args[child.Key()]
				if ok {
					sb.WriteString(stringz.SingleQuote(val))
					break
				}
			}

			// It's an error if the arg is not supplied.
			return "", errz.Errorf("no --arg value found for query variable %s", child.Text())
		case *ast.ExprNode:
			val, err := r.Expr(rc, child)
			if err != nil {
				return "", err
			}
			sb.WriteString(val)
		case *ast.LiteralNode:
			val, err := r.Literal(rc, child)
			if err != nil {
				return "", err
			}
			sb.WriteString(val)
		case *ast.FuncNode:
			val, err := r.Function(rc, child)
			if err != nil {
				return "", err
			}
			sb.WriteString(val)
		default:
			// FIXME: Should log a warning here
			// Shouldn't happen? Need to investigate.
			sb.WriteString(child.Text())
		}
	}

	if expr.HasParens() {
		sb.WriteRune(')')
	}

	return sb.String(), nil
}
