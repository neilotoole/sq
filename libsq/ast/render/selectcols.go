package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
)

func doSelectCols(rc *Context, cols []ast.ResultColumn) (string, error) {
	var err error

	if len(cols) == 0 {
		return "*", nil
	}

	vals := make([]string, len(cols))
	for i, col := range cols {
		// aliasFrag holds the "AS alias" fragment (if applicable).
		// For example: "@sakila | .actor | .first_name:given_name" becomes
		// "SELECT first_name AS given_name FROM actor".
		var aliasFrag string
		if col.Alias() != "" {
			aliasFrag = " AS " + rc.Dialect.Enquote(col.Alias())
		}

		switch col := col.(type) {
		case *ast.ColSelectorNode:
			if vals[i], err = renderSelectorNode(rc.Dialect, col); err != nil {
				return "", err
			}
		case *ast.TblColSelectorNode:
			if vals[i], err = renderSelectorNode(rc.Dialect, col); err != nil {
				return "", err
			}
		case *ast.FuncNode:
			if vals[i], err = rc.Renderer.Function(rc, col); err != nil {
				return "", err
			}
		case *ast.ExprElementNode:
			if vals[i], err = rc.Renderer.Expr(rc, col.ExprNode()); err != nil {
				return "", err
			}
		default:
			// FIXME: We should be exhaustively checking the cases.
			// Actually, this should probably be an error?
			vals[i] = col.Text() // for now, we just return the raw text
		}

		vals[i] += aliasFrag
	}

	text := strings.Join(vals, ", ")
	return text, nil
}
