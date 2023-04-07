package render

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
)

func doSelectCols(bc *BuildContext, r *Renderer, cols []ast.ResultColumn) (string, error) {
	quote := string(bc.Dialect.IdentQuote)

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
			aliasFrag = " AS " + bc.Dialect.Enquote(col.Alias())
		}

		switch col := col.(type) {
		// FIXME: switch to using renderSelectorNode.
		case *ast.ColSelectorNode:
			vals[i] = fmt.Sprintf("%s%s%s", quote, col.ColName(), quote)
		case *ast.TblColSelectorNode:
			vals[i] = fmt.Sprintf("%s%s%s.%s%s%s", quote, col.TblName(), quote, quote, col.ColName(), quote)
		case *ast.FuncNode:
			// it's a function
			var err error
			if vals[i], err = r.Function(bc, r, col); err != nil {
				return "", err
			}
		default:
			// FIXME: We should be exhaustively checking the cases.
			// Here, it's probably an ExprNode?
			vals[i] = col.Text() // for now, we just return the raw text
		}

		vals[i] += aliasFrag
	}

	text := strings.Join(vals, ", ")
	return text, nil
}
