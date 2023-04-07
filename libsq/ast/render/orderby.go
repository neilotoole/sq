package render

import (
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

	clause := "ORDER BY "
	for i := 0; i < len(terms); i++ {
		if i > 0 {
			clause += ", "
		}

		sel, err := renderSelectorNode(string(rc.Dialect.IdentQuote), terms[i].Selector())
		if err != nil {
			return "", err
		}

		clause += sel
		switch terms[i].Direction() { //nolint:exhaustive
		case ast.OrderByDirectionAsc:
			clause += " ASC"
		case ast.OrderByDirectionDesc:
			clause += " DESC"
		default:
		}
	}

	return clause, nil
}
