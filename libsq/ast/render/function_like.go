package render

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// LikeMode identifies which substring-matching shape is being rendered.
type LikeMode string

const (
	LikeContains   LikeMode = "contains"
	LikeStartsWith LikeMode = "startswith"
	LikeEndsWith   LikeMode = "endswith"
)

// likeEscapeChar is the ESCAPE character emitted for LIKE patterns produced
// by contains/startswith/endswith. We use `|` rather than `\` because `\`
// is interpreted by MySQL's string-literal parser (unless NO_BACKSLASH_ESCAPES
// is set), which would corrupt the pattern. `|` is uncommon in user search
// strings and is treated literally inside SQL string literals on every
// supported driver.
const likeEscapeChar = '|'

// likeEscapeClause is the SQL fragment emitted after the pattern literal to
// declare the escape character. Derived from likeEscapeChar so the two stay
// in lock-step.
var likeEscapeClause = " ESCAPE '" + string(likeEscapeChar) + "'"

// escapeLikePattern prefixes likeEscapeChar before each LIKE meta-character
// (% and _) and before any literal occurrence of the escape char itself.
// extraMeta lists additional characters the driver treats as LIKE meta-chars
// and that must therefore be escaped (e.g. "[" and "]" for SQL Server's
// character classes). Pass "" for dialects whose LIKE meta-chars are limited
// to % and _. Passing characters in extraMeta that overlap with %, _, or the
// escape char is harmless: the built-in branch matches first.
func escapeLikePattern(s, extraMeta string) string {
	if !strings.ContainsAny(s, "%_|"+extraMeta) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		if r == '%' || r == '_' || r == likeEscapeChar || strings.ContainsRune(extraMeta, r) {
			b.WriteRune(likeEscapeChar)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// buildLikePattern returns the LIKE-pattern string (without surrounding
// quotes or the ESCAPE clause) for the given mode and (already-unquoted)
// literal value. See [escapeLikePattern] for extraMeta.
func buildLikePattern(s string, mode LikeMode, extraMeta string) string {
	escaped := escapeLikePattern(s, extraMeta)
	switch mode {
	case LikeContains:
		return "%" + escaped + "%"
	case LikeStartsWith:
		return escaped + "%"
	case LikeEndsWith:
		return "%" + escaped
	default:
		panic("unreachable: invalid LikeMode " + string(mode))
	}
}

// ParseLikeArgs validates the shape of a substring-matching function call
// (contains/startswith/endswith) and returns the rendered column SQL and
// the unquoted literal value. The first argument must be a column selector;
// the second must be a quoted string literal. Other shapes are rejected
// with a clear error. Exported for use by driver-specific overrides.
func ParseLikeArgs(rc *Context, fn *ast.FuncNode) (colSQL, literal string, err error) {
	children := fn.Children()
	if len(children) != 2 {
		return "", "", errz.Errorf(
			"%s() requires exactly 2 arguments (column, pattern), got %d",
			fn.FuncName(), len(children))
	}

	// SLQ parses function arguments as expression trees, so each child is
	// typically wrapped in an *ast.ExprNode. Peel through single-child
	// wrappers to reach the underlying selector and literal leaves; reject
	// anything with internal branching.
	colNode, ok := ast.NodeUnwrap[ast.Node](children[0])
	if !ok {
		return "", "", errz.Errorf(
			"%s() first argument must be a column selector", fn.FuncName())
	}
	colSQL, err = renderSelectorNode(rc.Dialect, colNode)
	if err != nil {
		return "", "", errz.Wrapf(err, "%s() first argument", fn.FuncName())
	}

	litNode, ok := ast.NodeUnwrap[*ast.LiteralNode](children[1])
	if !ok {
		return "", "", errz.Errorf(
			"%s() second argument must be a string literal", fn.FuncName())
	}
	val, wasQuoted, err := unquoteLiteral(litNode.Text())
	if err != nil {
		return "", "", errz.Wrapf(err, "%s() second argument", fn.FuncName())
	}
	if !wasQuoted {
		return "", "", errz.Errorf(
			"%s() second argument must be a quoted string literal",
			fn.FuncName())
	}
	return colSQL, val, nil
}

// LikeOpts configures [RenderLikeOp]. Only Mode is required.
type LikeOpts struct {
	// Mode selects which substring-matching shape to render.
	Mode LikeMode

	// Op is the LIKE operator. Defaults to "LIKE"; MySQL uses "LIKE BINARY"
	// to force byte-level comparison.
	Op string

	// ColCollate, when non-empty, is appended verbatim after the column
	// reference (e.g. " COLLATE Latin1_General_BIN2" — note the leading
	// space). Callers must include the leading space.
	ColCollate string

	// ExtraMeta lists characters the driver treats as LIKE meta-chars in
	// addition to % and _ (e.g. "[]" for SQL Server's character classes).
	ExtraMeta string
}

// RenderLikeOp renders the LIKE-based shape:
//
//	<colSQL><opts.ColCollate> <opts.Op> '<pattern>' ESCAPE '<likeEscapeChar>'
//
// Used by the default-renderer overrides and by MySQL/SQL Server overrides.
// SQLite and ClickHouse use different shapes and do not call this function.
func RenderLikeOp(rc *Context, fn *ast.FuncNode, opts LikeOpts) (string, error) {
	colSQL, lit, err := ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	op := opts.Op
	if op == "" {
		op = "LIKE"
	}
	pattern := buildLikePattern(lit, opts.Mode, opts.ExtraMeta)
	return colSQL + opts.ColCollate + " " + op + " " + stringz.SingleQuote(pattern) + likeEscapeClause, nil
}

func doFuncContains(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeContains})
}

func doFuncStartsWith(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeStartsWith})
}

func doFuncEndsWith(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeEndsWith})
}
