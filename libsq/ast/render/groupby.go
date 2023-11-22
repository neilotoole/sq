package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func doGroupBy(rc *Context, gb *ast.GroupByNode) (string, error) {
	if gb == nil {
		return "", nil
	}

	var (
		term string
		err  error
		sb   strings.Builder
	)

	sb.WriteString("GROUP BY ")
	for i, child := range gb.Children() {
		if i > 0 {
			sb.WriteString(", ")
		}

		switch child := child.(type) {
		case *ast.FuncNode:
			if term, err = rc.Renderer.Function(rc, child); err != nil {
				return "", err
			}
		case ast.Selector:
			if term, err = renderSelectorNode(rc.Dialect, child); err != nil {
				return "", err
			}
		default:
			// Should never happen
			return "", errz.Errorf("invalid child type: %T: %s", child, child)
		}

		sb.WriteString(term)
	}

	return sb.String(), nil
}

func doHaving(rc *Context, hn *ast.HavingNode) (string, error) {
	if hn == nil {
		return "", nil
	}

	var (
		err error
		sb  strings.Builder
	)

	sb.WriteString("HAVING ")

	children := hn.Children()
	if len(children) != 1 {
		return "", errz.Errorf("having() clause should have exactly one child, but has %d",
			len(children))
	}

	exprNode, ok := children[0].(*ast.ExprNode)
	if !ok {
		return "", errz.Errorf("having() clause child should be of type {%T}, but is {%T}",
			exprNode, children[0])
	}

	exprFrag, err := rc.Renderer.Expr(rc, exprNode)
	if err != nil {
		return "", err
	}
	sb.WriteString(exprFrag)
	return sb.String(), nil
}
