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

	// Op is the LIKE operator. Defaults to "LIKE". Drivers force
	// case-sensitivity here (MySQL "LIKE BINARY") or case-insensitivity
	// (Postgres "ILIKE").
	Op string

	// ColCollate, when non-empty, is appended verbatim after the column
	// reference (e.g. " COLLATE Latin1_General_BIN2" — note the leading
	// space). Callers must include the leading space.
	ColCollate string

	// ExtraMeta lists characters the driver treats as LIKE meta-chars
	// in addition to % and _ (e.g. "[]" for SQL Server).
	ExtraMeta string

	// IgnoreCase, when true, wraps the column and literal in LOWER(...).
	// IgnoreCase is mutually exclusive with non-empty Op and ColCollate:
	// LOWER-wrapping handles case-insensitivity portably without needing
	// a driver-specific operator or collation, and combining the
	// strategies produces redundant or semantically incoherent SQL
	// (e.g. LOWER(col) COLLATE Latin1_General_CI_AS LIKE LOWER(pat)).
	// Combining them panics at render time.
	IgnoreCase bool
}

// RenderLikeOp renders the LIKE-based shape:
//
//	<colSQL><opts.ColCollate> <opts.Op> '<pattern>' ESCAPE '<likeEscapeChar>'
//
// When opts.IgnoreCase is true, both <colSQL> and the quoted pattern
// are wrapped in LOWER(...) before assembly.
//
// Used by the default-renderer overrides and by MySQL/SQL Server overrides.
// SQLite and ClickHouse use different shapes and do not call this function.
//
// opts.IgnoreCase is mutually exclusive with opts.Op and opts.ColCollate:
// the LOWER-wrapping strategy stands alone, and combining it with a
// non-default operator or collation panics at render time.
func RenderLikeOp(rc *Context, fn *ast.FuncNode, opts LikeOpts) (string, error) {
	if opts.IgnoreCase && (opts.Op != "" || opts.ColCollate != "") {
		panic("RenderLikeOp: IgnoreCase is mutually exclusive with Op and ColCollate")
	}
	colSQL, lit, err := ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	op := opts.Op
	if op == "" {
		op = "LIKE"
	}
	pattern := buildLikePattern(lit, opts.Mode, opts.ExtraMeta)
	litSQL := stringz.SingleQuote(pattern)
	if opts.IgnoreCase {
		colSQL = "LOWER(" + colSQL + ")"
		litSQL = "LOWER(" + litSQL + ")"
	}
	return colSQL + opts.ColCollate + " " + op + " " + litSQL + likeEscapeClause, nil
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

func doFuncIContains(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeContains, IgnoreCase: true})
}

func doFuncIStartsWith(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeStartsWith, IgnoreCase: true})
}

func doFuncIEndsWith(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeEndsWith, IgnoreCase: true})
}

func doFuncLike(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeRaw(rc, fn, LikeRawOpts{})
}

func doFuncILike(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeRaw(rc, fn, LikeRawOpts{IgnoreCase: true})
}

// LikeRawOpts configures [RenderLikeRaw], the user-wildcard-controlled
// shape used by SLQ's like / ilike functions.
type LikeRawOpts struct {
	// Op is the LIKE operator. Defaults to "LIKE". Drivers force
	// case-sensitivity here (MySQL "LIKE BINARY") or case-insensitivity
	// (Postgres "ILIKE").
	Op string

	// ColCollate, when non-empty, is appended verbatim after the column
	// reference (e.g. " COLLATE Latin1_General_BIN2" — note the leading
	// space). Callers must include the leading space.
	ColCollate string

	// OmitEscape, when true, suppresses the trailing " ESCAPE '|'"
	// clause. Used for drivers (e.g. ClickHouse) that don't support
	// an ESCAPE clause on LIKE.
	OmitEscape bool

	// IgnoreCase, when true, wraps the column and literal in LOWER(...).
	// IgnoreCase is mutually exclusive with non-empty Op and ColCollate:
	// LOWER-wrapping handles case-insensitivity portably without needing
	// a driver-specific operator or collation, and combining the
	// strategies produces redundant or semantically incoherent SQL
	// (e.g. LOWER(col) COLLATE Latin1_General_CI_AS LIKE LOWER(pat)).
	// Combining them panics at render time.
	IgnoreCase bool
}

// RenderLikeRaw renders the user-controlled LIKE shape used by SLQ's
// like / ilike functions. Unlike [RenderLikeOp], the literal pattern
// is bound verbatim: % and _ are wildcards, not escaped. Single
// quotes inside the literal are still properly escaped by
// SingleQuote.
//
// opts.IgnoreCase is mutually exclusive with opts.Op and opts.ColCollate:
// the LOWER-wrapping strategy stands alone, and combining it with a
// non-default operator or collation panics at render time.
func RenderLikeRaw(rc *Context, fn *ast.FuncNode, opts LikeRawOpts) (string, error) {
	if opts.IgnoreCase && (opts.Op != "" || opts.ColCollate != "") {
		panic("RenderLikeRaw: IgnoreCase is mutually exclusive with Op and ColCollate")
	}
	colSQL, lit, err := ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	op := opts.Op
	if op == "" {
		op = "LIKE"
	}
	litSQL := stringz.SingleQuote(lit)
	if opts.IgnoreCase {
		colSQL = "LOWER(" + colSQL + ")"
		litSQL = "LOWER(" + litSQL + ")"
	}
	out := colSQL + opts.ColCollate + " " + op + " " + litSQL
	if !opts.OmitEscape {
		out += likeEscapeClause
	}
	return out, nil
}
