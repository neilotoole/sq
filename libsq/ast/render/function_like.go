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

// parseLikeColArg is the shared LHS parser for the LIKE-family
// arg-validation helpers. It checks the arg count, renders the first
// argument (which must be a column selector), and returns the raw RHS
// child node for the caller to unwrap and dispatch on. The two callers
// (ParseLikeArgs and ParseLikePatternArgs) have different RHS contracts,
// so RHS handling stays in each caller.
//
// SLQ parses function arguments as expression trees, so each child is
// typically wrapped in an *ast.ExprNode. NodeUnwrap peels those
// single-child wrappers to reach the underlying selector / literal
// leaves; nodes with internal branching are rejected.
func parseLikeColArg(rc *Context, fn *ast.FuncNode) (colSQL string, rhsChild ast.Node, err error) {
	children := fn.Children()
	if len(children) != 2 {
		return "", nil, errz.Errorf(
			"%s() requires exactly 2 arguments (column, pattern), got %d",
			fn.FuncName(), len(children))
	}
	colNode, ok := ast.NodeUnwrap[ast.Node](children[0])
	if !ok {
		return "", nil, errz.Errorf(
			"%s() first argument must be a column selector", fn.FuncName())
	}
	colSQL, err = renderSelectorNode(rc.Dialect, colNode)
	if err != nil {
		return "", nil, errz.Wrapf(err, "%s() first argument", fn.FuncName())
	}
	return colSQL, children[1], nil
}

// ParseLikeArgs validates the shape of a substring-matching function call
// (contains/startswith/endswith and their case-insensitive companions)
// and returns the rendered column SQL and the unquoted literal value.
// The first argument must be a column selector; the second must be a
// quoted string literal. Other shapes are rejected with a clear error.
// Exported for use by driver-specific overrides.
//
// For like / ilike, which additionally accept a column selector as the
// second argument, use [ParseLikePatternArgs].
func ParseLikeArgs(rc *Context, fn *ast.FuncNode) (colSQL, literal string, err error) {
	colSQL, rhsChild, err := parseLikeColArg(rc, fn)
	if err != nil {
		return "", "", err
	}
	litNode, ok := ast.NodeUnwrap[*ast.LiteralNode](rhsChild)
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

// ParseLikePatternArgs is the like / ilike companion to [ParseLikeArgs].
// It validates the shape of a user-controlled-wildcard function call and
// returns the rendered column SQL for the first argument plus the
// rendered SQL for the second argument, which may be either a quoted
// string literal (returned single-quoted, ready to splice into the
// emitted LIKE clause) or a column selector (rendered via the dialect's
// selector renderer). Mixed-form calls — like(.col, "lit") and
// like(.col, .other_col) — are both accepted and dispatch on the
// second-argument node type at parse time. Exported for use by
// driver-specific overrides.
func ParseLikePatternArgs(rc *Context, fn *ast.FuncNode) (colSQL, rhsSQL string, err error) {
	colSQL, rhsChild, err := parseLikeColArg(rc, fn)
	if err != nil {
		return "", "", err
	}
	rhsNode, ok := ast.NodeUnwrap[ast.Node](rhsChild)
	if !ok {
		return "", "", errz.Errorf(
			"%s() second argument must be a string literal or column selector",
			fn.FuncName())
	}

	if lit, isLit := rhsNode.(*ast.LiteralNode); isLit {
		val, wasQuoted, errLit := unquoteLiteral(lit.Text())
		if errLit != nil {
			return "", "", errz.Wrapf(errLit, "%s() second argument", fn.FuncName())
		}
		if !wasQuoted {
			return "", "", errz.Errorf(
				"%s() second argument must be a quoted string literal or column selector",
				fn.FuncName())
		}
		return colSQL, stringz.SingleQuote(val), nil
	}

	rhsSQL, err = renderSelectorNode(rc.Dialect, rhsNode)
	if err != nil {
		return "", "", errz.Wrapf(err,
			"%s() second argument must be a string literal or column selector",
			fn.FuncName())
	}
	return colSQL, rhsSQL, nil
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

// RegisterILikeFamily registers the four case-insensitive matchers
// (icontains, istartswith, iendswith, ilike) on r to use native
// ILIKE rather than the default LOWER(col) LIKE LOWER(pat) shape.
// Used by drivers whose dialect supports ILIKE (currently Postgres
// and DuckDB).
func RegisterILikeFamily(r *Renderer) {
	r.FunctionOverrides[ast.FuncNameIContains] = doFuncIContainsILike
	r.FunctionOverrides[ast.FuncNameIStartsWith] = doFuncIStartsWithILike
	r.FunctionOverrides[ast.FuncNameIEndsWith] = doFuncIEndsWithILike
	r.FunctionOverrides[ast.FuncNameILike] = doFuncILikeILike
}

func doFuncIContainsILike(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeContains, Op: "ILIKE"})
}

func doFuncIStartsWithILike(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeStartsWith, Op: "ILIKE"})
}

func doFuncIEndsWithILike(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeOp(rc, fn, LikeOpts{Mode: LikeEndsWith, Op: "ILIKE"})
}

func doFuncILikeILike(rc *Context, fn *ast.FuncNode) (string, error) {
	return RenderLikeRaw(rc, fn, LikeRawOpts{Op: "ILIKE"})
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
// like / ilike functions. Unlike [RenderLikeOp], the pattern is bound
// verbatim: % and _ are wildcards, not auto-escaped. The pattern may
// be either a quoted string literal (single quotes inside are escaped
// by SingleQuote) or a column selector (resolved per-row at execution
// time). See [ParseLikePatternArgs] for the dispatch.
//
// No `ESCAPE '|'` clause is emitted, so `|` is a literal character on
// every driver. Other engine-default escape semantics remain
// driver-specific — notably MySQL's default backslash escape (`\`)
// still applies in `LIKE` patterns unless the session sets the
// `NO_BACKSLASH_ESCAPES` SQL mode. Users who need literal `%` / `_`
// matching should use [RenderLikeOp] (SLQ's contains family), which
// auto-escapes wildcards in the pattern.
//
// opts.IgnoreCase is mutually exclusive with opts.Op and opts.ColCollate:
// the LOWER-wrapping strategy stands alone, and combining it with a
// non-default operator or collation panics at render time.
func RenderLikeRaw(rc *Context, fn *ast.FuncNode, opts LikeRawOpts) (string, error) {
	if opts.IgnoreCase && (opts.Op != "" || opts.ColCollate != "") {
		panic("RenderLikeRaw: IgnoreCase is mutually exclusive with Op and ColCollate")
	}
	colSQL, rhsSQL, err := ParseLikePatternArgs(rc, fn)
	if err != nil {
		return "", err
	}
	op := opts.Op
	if op == "" {
		op = "LIKE"
	}
	if opts.IgnoreCase {
		colSQL = "LOWER(" + colSQL + ")"
		rhsSQL = "LOWER(" + rhsSQL + ")"
	}
	return colSQL + opts.ColCollate + " " + op + " " + rhsSQL, nil
}
