package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/kind"
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
			// If the driver declares a forced result kind for this function
			// (e.g. SQLite/rqlite pin sum() to kind.Decimal because the backend
			// reports no usable type for it), record it against this output
			// column position for the driver to apply when building record
			// metadata. See issue #839.
			//
			// The hint is keyed by output-column index i; the driver applies it
			// to colTypes[i], so this relies on the rendered SELECT column order
			// matching the result column order one-to-one (it does today: i
			// indexes qm.Cols, the ordered result columns). It only fires for a
			// function used directly as a result column. A function nested inside
			// an expression (the ExprElementNode case below, e.g. sum(.x)+0) is
			// not pinned, so on SQLite/rqlite such a result falls back to the
			// scanned int/float kind; the cast-based drivers still get their type
			// because the cast travels with the function rendering.
			if knd, ok := rc.Renderer.FunctionResultKinds[strings.ToLower(col.FuncName())]; ok {
				if rc.ResultColumnKinds == nil {
					rc.ResultColumnKinds = make(map[int]kind.Kind)
				}
				rc.ResultColumnKinds[i] = knd
			}
		case *ast.ExprElementNode:
			if vals[i], err = rc.Renderer.Expr(rc, col.ExprNode()); err != nil {
				return "", err
			}
		default:
			// REVISIT: We should be exhaustively checking the cases.
			// Actually, this should probably be an error?
			vals[i] = col.Text() // for now, we just return the raw text
		}

		vals[i] += aliasFrag
	}

	text := strings.Join(vals, ", ")
	return text, nil
}
