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
			if term, err = renderSelectorNode(string(rc.Dialect.IdentQuote), child); err != nil {
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
