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

// EscapeLikePattern prefixes likeEscapeChar before each LIKE meta-character
// (% and _) and before any literal occurrence of the escape char itself.
// Exported for use by driver-specific overrides.
func EscapeLikePattern(s string) string {
	if !strings.ContainsAny(s, "%_|") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 4)
	for _, r := range s {
		if r == '%' || r == '_' || r == likeEscapeChar {
			b.WriteRune(likeEscapeChar)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// BuildLikePattern returns the LIKE-pattern string (without surrounding
// quotes or the ESCAPE clause) for the given mode and (already-unquoted)
// literal value. Exported for use by driver-specific overrides.
func BuildLikePattern(s string, mode LikeMode) string {
	escaped := EscapeLikePattern(s)
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

	// The parser commonly wraps function arguments in an *ast.ExprNode.
	// Unwrap to get the underlying leaf node.
	colNode := unwrapSingleChild(children[0])
	colSQL, err = renderSelectorNode(rc.Dialect, colNode)
	if err != nil {
		return "", "", errz.Wrapf(err,
			"%s() first argument must be a column selector", fn.FuncName())
	}

	litNodeRaw := unwrapSingleChild(children[1])
	litNode, ok := litNodeRaw.(*ast.LiteralNode)
	if !ok {
		return "", "", errz.Errorf(
			"%s() second argument must be a string literal, got %T",
			fn.FuncName(), litNodeRaw)
	}
	val, wasQuoted, err := unquoteLiteral(litNode.Text())
	if err != nil {
		return "", "", err
	}
	if !wasQuoted {
		return "", "", errz.Errorf(
			"%s() second argument must be a quoted string literal",
			fn.FuncName())
	}
	return colSQL, val, nil
}

// unwrapSingleChild unwraps node if it is an *ast.ExprNode containing a
// single child, recursively. This mirrors the convention used elsewhere
// in the renderer where function arguments may be wrapped in an ExprNode.
func unwrapSingleChild(node ast.Node) ast.Node {
	for {
		expr, ok := node.(*ast.ExprNode)
		if !ok {
			return node
		}
		kids := expr.Children()
		if len(kids) != 1 {
			return node
		}
		node = kids[0]
	}
}

// RenderLikeOp renders the LIKE-based shape:
//
//	<colSQL><colCollate> <likeOp> '<pattern>' ESCAPE '|'
//
// likeOp is typically "LIKE" or "LIKE BINARY" (MySQL).
// colCollate, when non-empty, is appended verbatim after the column reference
// (e.g. " COLLATE Latin1_General_BIN2" — note the leading space). It's the
// caller's responsibility to include the leading space.
//
// Used by the default-renderer overrides and by MySQL/SQL Server overrides.
// SQLite uses a different shape and does not call this function.
func RenderLikeOp(rc *Context, fn *ast.FuncNode, mode LikeMode, likeOp, colCollate string) (string, error) {
	colSQL, lit, err := ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	pattern := BuildLikePattern(lit, mode)
	return colSQL + colCollate + " " + likeOp + " " + stringz.SingleQuote(pattern) + " ESCAPE '|'", nil
}

func doFuncContains(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeContains, "LIKE", "")
}

func doFuncStartsWith(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeStartsWith, "LIKE", "")
}

func doFuncEndsWith(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeEndsWith, "LIKE", "")
}
