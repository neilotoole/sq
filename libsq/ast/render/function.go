package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

func doFunction(rc *Context, fn *ast.FuncNode) (string, error) {
	sb := strings.Builder{}
	fnName := strings.ToLower(fn.FuncName())

	if f, ok := rc.Renderer.FunctionOverrides[fnName]; ok {
		// The SQL function name has a custom renderer.
		return f(rc, fn)
	}

	if f, ok := rc.Renderer.FunctionNames[fnName]; ok {
		// The SLQ function name is mapped to a different SQL function
		// for this dialect.
		fnName = f
	}
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

// FuncRowNum renders the rownum() function.
// FIXME: make this private, and add to the default renderer.
func FuncRowNum(rc *Context, fn *ast.FuncNode) (string, error) {
	a, _ := ast.NodeRoot(fn).(*ast.AST)
	obNode := ast.FindFirstNode[*ast.OrderByNode](a)
	if obNode != nil {
		obClause, err := rc.Renderer.OrderBy(rc, obNode)
		if err != nil {
			return "", err
		}
		return "(row_number() OVER (" + obClause + "))", nil
	}

	// It's not entirely clear that this "ORDER BY 1" mechanism
	// is the correct approach, but it seems to work for SQLite and Postgres.
	return "(row_number() OVER (ORDER BY 1))", nil
}
