package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func doOrderBy(rc *Context, ob *ast.OrderByNode) (string, error) {
	if ob == nil {
		return "", nil
	}

	terms := ob.Terms()
	if len(terms) == 0 {
		return "", errz.Errorf("%T has no ordering terms: %s", ob, ob)
	}

	var sb strings.Builder
	sb.WriteString("ORDER BY ")
	for i := range terms {
		if i > 0 {
			sb.WriteString(", ")
		}

		sel, err := renderSelectorNode(rc.Dialect, terms[i].Selector())
		if err != nil {
			return "", err
		}

		sb.WriteString(sel)
		switch terms[i].Direction() { //nolint:exhaustive
		case ast.OrderByDirectionAsc:
			sb.WriteString(" ASC")
		case ast.OrderByDirectionDesc:
			sb.WriteString(" DESC")
		default:
		}
	}

	return sb.String(), nil
}
