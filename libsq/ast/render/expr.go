package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// Expr implements FragmentBuilder.
func doExpr(rc *Context, expr *ast.ExprNode) (string, error) {
	if expr == nil {
		return "", nil
	}
	r := rc.Renderer

	var sb strings.Builder
	for i, child := range expr.Children() {
		if i > 0 {
			sb.WriteRune(sp)
		}

		switch child := child.(type) {
		case *ast.TblColSelectorNode, *ast.ColSelectorNode:
			val, err := renderSelectorNode(string(rc.Dialect.IdentQuote), child)
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
		default:
			sb.WriteString(child.Text())
		}
	}

	return sb.String(), nil
}
