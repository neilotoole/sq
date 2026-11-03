package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/vector"
)

func doSelectCols(rc *Context, cols []ast.ResultColumn) (string, error) {
	var err error

	if len(cols) == 0 {
		// SELECT * returns the table's visible columns in declaration order.
		if h := rc.vectorTableHints(); h != nil {
			for i, name := range h.Columns {
				rc.setResultColumnVector(i, h, name)
			}
		}
		return "*", nil
	}

	vecHints := rc.vectorTableHints()

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
			// An alias doesn't change the source column, so the vector type
			// is looked up by the column name.
			rc.setResultColumnVector(i, vecHints, col.ColName())
		case *ast.TblColSelectorNode:
			if vals[i], err = renderSelectorNode(rc.Dialect, col); err != nil {
				return "", err
			}
			rc.setResultColumnVector(i, vecHints, col.ColName())
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

// vectorTableHints returns the hints for rc.VectorTable, or nil if there's no
// vector table.
func (rc *Context) vectorTableHints() *TableHints {
	if rc.VectorTable == "" {
		return nil
	}
	h := rc.TableHints[rc.VectorTable]
	if h == nil || len(h.Vectors) == 0 {
		return nil
	}
	return h
}

// setResultColumnVector records the vector type of colName for output
// position i in rc.ResultColumnVectors, if colName is a vector column in h.
// SQLite resolves column names case-insensitively, so an exact match is
// preferred, falling back to a case-insensitive one. A nil h is a no-op.
func (rc *Context) setResultColumnVector(i int, h *TableHints, colName string) {
	if h == nil || colName == "" {
		return
	}

	vt, ok := h.Vectors[colName]
	if !ok {
		for name, t := range h.Vectors {
			if strings.EqualFold(name, colName) {
				vt, ok = t, true
				break
			}
		}
	}
	if !ok {
		return
	}

	if rc.ResultColumnVectors == nil {
		rc.ResultColumnVectors = make(map[int]vector.Type)
	}
	rc.ResultColumnVectors[i] = vt
}
