package rqlite

import (
	"strconv"
	"unicode/utf8"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// The SLQ-function renderers below are SQLite-flavored and identical to
// the sqlite3 driver's. Since rqlite executes SQLite SQL under the
// hood, the same rendering applies verbatim.

func renderFuncContainsInstr(rc *render.Context, fn *ast.FuncNode) (string, error) {
	colSQL, lit, err := render.ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	return "instr(" + colSQL + ", " + stringz.SingleQuote(lit) + ") > 0", nil
}

func renderFuncStartsWithSubstr(rc *render.Context, fn *ast.FuncNode) (string, error) {
	colSQL, lit, err := render.ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	n := utf8.RuneCountInString(lit)
	return "substr(" + colSQL + ", 1, " + strconv.Itoa(n) + ") = " + stringz.SingleQuote(lit), nil
}

func renderFuncEndsWithSubstr(rc *render.Context, fn *ast.FuncNode) (string, error) {
	colSQL, lit, err := render.ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	n := utf8.RuneCountInString(lit)
	if n == 0 {
		// SQLite evaluates substr(col, -0) as substr(col, 0), which
		// returns the full string, so `substr(col, -N) = ''` would be
		// false for every row. Emit `col LIKE '%'` to match the
		// LIKE-based drivers exactly, including NULL propagation under
		// negation.
		return colSQL + " LIKE '%'", nil
	}
	return "substr(" + colSQL + ", -" + strconv.Itoa(n) + ") = " + stringz.SingleQuote(lit), nil
}

// SQLite's default LIKE is ASCII case-insensitive, so the i* family
// uses plain LIKE rather than the instr/substr shape that the
// case-sensitive variants use.

func renderFuncIContainsLike(rc *render.Context, fn *ast.FuncNode) (string, error) {
	return render.RenderLikeOp(rc, fn, render.LikeOpts{Mode: render.LikeContains})
}

func renderFuncIStartsWithLike(rc *render.Context, fn *ast.FuncNode) (string, error) {
	return render.RenderLikeOp(rc, fn, render.LikeOpts{Mode: render.LikeStartsWith})
}

func renderFuncIEndsWithLike(rc *render.Context, fn *ast.FuncNode) (string, error) {
	return render.RenderLikeOp(rc, fn, render.LikeOpts{Mode: render.LikeEndsWith})
}

// renderFuncLike renders SLQ's like and ilike. SQLite's default LIKE
// is ASCII case-insensitive, so the two functions are structurally
// indistinguishable on this driver and both register this same helper.
func renderFuncLike(rc *render.Context, fn *ast.FuncNode) (string, error) {
	return render.RenderLikeRaw(rc, fn, render.LikeRawOpts{})
}
