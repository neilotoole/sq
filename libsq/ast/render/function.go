package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

func doFunction(rc *Context, fn *ast.FuncNode) (string, error) {
	fnName := strings.ToLower(fn.FuncName())
	if f, ok := rc.Renderer.FunctionOverrides[fnName]; ok {
		return f(rc, fn)
	}
	return RenderFuncDefault(rc, fn)
}

// RenderFuncDefault renders fn using the default function-render logic,
// bypassing Renderer.FunctionOverrides. It's intended for FunctionOverrides
// implementations that need to wrap the default rendering — e.g. wrapping
// a SQL function call in a cast.
func RenderFuncDefault(rc *Context, fn *ast.FuncNode) (string, error) {
	sb := strings.Builder{}
	fnName := strings.ToLower(fn.FuncName())

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

		s, err := RenderFuncArg(rc, child)
		if err != nil {
			return "", err
		}
		sb.WriteString(s)
	}

	sb.WriteRune(')')
	sql := sb.String()
	return sql, nil
}

// RenderFuncArg renders a single function-argument node to SQL. It's exported
// so that a driver's FunctionOverrides impl can render an operand in isolation,
// for example to wrap it in a CAST. See the SQL Server avg() override, which
// emits avg(CAST(col AS FLOAT)) to defeat integer-AVG truncation.
func RenderFuncArg(rc *Context, node ast.Node) (string, error) {
	switch node := node.(type) {
	case *ast.ColSelectorNode, *ast.TblColSelectorNode, *ast.TblSelectorNode:
		return renderSelectorNode(rc.Dialect, node)
	case *ast.OperatorNode:
		return node.Text(), nil
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
			return stringz.SingleQuote(val), nil
		}
		return val, nil
	case *ast.ExprNode:
		return rc.Renderer.Expr(rc, node)
	default:
		return "", errz.Errorf("unknown AST child node %T: %s", node, node)
	}
}

// doFuncRowNum renders the rownum() function.
func doFuncRowNum(rc *Context, fn *ast.FuncNode) (string, error) {
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
