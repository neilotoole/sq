package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func doFunction(rc *Context, fn *ast.FuncNode) (string, error) {
	sb := strings.Builder{}
	fnName := strings.ToLower(fn.FuncName())
	children := fn.Children()

	if len(children) == 0 {
		sb.WriteString(fnName)
		sb.WriteRune('(')

		if fnName == "count" {
			// Special handling for the count function, because COUNT()
			// isn't valid, but COUNT(*) is.
			sb.WriteRune('*')
		}

		sb.WriteRune(')')
		return sb.String(), nil
	}

	// Special handling for "count_unique(.col)" function. We translate
	// it to "SELECT count(DISTINCT col)".
	if fnName == "count_unique" {
		sb.WriteString("count(DISTINCT ")
	} else {
		sb.WriteString(fnName)
		sb.WriteRune('(')
	}

	for i, child := range children {
		if i > 0 {
			sb.WriteString(", ")
		}

		switch node := child.(type) {
		case *ast.ColSelectorNode, *ast.TblColSelectorNode, *ast.TblSelectorNode:
			s, err := renderSelectorNode(rc.Dialect, node)
			if err != nil {
				return "", err
			}
			sb.WriteString(s)
		case *ast.OperatorNode:
			sb.WriteString(node.Text())
		case *ast.LiteralNode:
			// TODO: This is all a bit of a mess. We probably need to
			// move to using bound parameters instead of inlining
			// literal values.
			val, wasQuoted, err := unquoteLiteral(node.Text())
			if err != nil {
				return "", err
			}

			if wasQuoted {
				// The literal had quotes, so it's a regular string.
				sb.WriteString(stringz.SingleQuote(val))
			} else {
				sb.WriteString(val)
			}
		case *ast.ExprNode:
			s, err := rc.Renderer.Expr(rc, node)
			if err != nil {
				return "", err
			}
			sb.WriteString(s)
		default:
			return "", errz.Errorf("unknown AST child node %T: %s", node, node)
		}
	}

	sb.WriteRune(')')
	sql := sb.String()
	return sql, nil
}
