// Package render provides the mechanism for rendering ast into SQL.
package render

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/driver/dialect"
)

// Context contains context for rendering a query.
type Context struct {
	// Renderer holds the rendering functions.
	Renderer *Renderer

	// The args map contains predefined variables that are
	// substituted into the query. It may be empty or nil.
	Args map[string]string

	// Fragments is the set of fragments that are rendered into
	// a SQL query. It may not be initialized until late in
	// the day.
	Fragments *Fragments

	// ResultColumnKinds records a forced kind.Kind for result columns by their
	// zero-based output position, populated during column rendering from
	// Renderer.FunctionResultKinds. It lets a driver pin the surfaced kind for a
	// function result that the backend cannot express in SQL: SQLite (and thus
	// rqlite) report no usable type for a sum() expression, so sum() is pinned to
	// kind.Decimal here and applied when the record metadata is built. See issue
	// #839. It's nil unless at least one result column has a forced kind.
	ResultColumnKinds map[int]kind.Kind

	// Dialect is the driver dialect.
	Dialect dialect.Dialect
}

// Renderer is a set of functions for rendering ast elements into SQL.
// Use NewDefaultRenderer to get a new instance. Each function can be
// swapped with a custom implementation for a SQL dialect.
type Renderer struct {
	// FromTable renders a FROM table fragment.
	FromTable func(rc *Context, tblSel *ast.TblSelectorNode) (string, error)

	// SelectCols renders a column names/expression fragment.
	// It shouldn't render the actual SELECT keyword. Example return value:
	//
	//   "first_name" AS "given_name", "last name" AS "family_name"
	SelectCols func(rc *Context, cols []ast.ResultColumn) (string, error)

	// Range renders a row range fragment.
	Range func(rc *Context, rr *ast.RowRangeNode) (string, error)

	// OrderBy renders the ORDER BY fragment.
	OrderBy func(rc *Context, ob *ast.OrderByNode) (string, error)

	// GroupBy renders the GROUP BY fragment.
	GroupBy func(rc *Context, gb *ast.GroupByNode) (string, error)

	// Having renders the HAVING fragment.
	Having func(rc *Context, having *ast.HavingNode) (string, error)

	// Join renders a join fragment.
	Join func(rc *Context, leftTbl *ast.TblSelectorNode, joins []*ast.JoinNode) (string, error)

	// Function renders a function fragment.
	Function func(rc *Context, fn *ast.FuncNode) (string, error)

	// FunctionNames is a map of SLQ function name to SQL function name.
	// It can be used by the Renderer.Function impl. Note that FunctionOverrides
	// has precedence over FunctionNames.
	FunctionNames map[string]string

	// FunctionOverrides is a map of SLQ function name to a custom
	// function to render that function. It can be used by the Renderer.Function
	// imp. FunctionOverrides has precedence over FunctionNames.
	FunctionOverrides map[string]func(rc *Context, fn *ast.FuncNode) (string, error)

	// FunctionResultKinds maps an SLQ function name to a kind.Kind that the
	// function's result column must be surfaced as, regardless of the type the
	// backend reports. It exists for drivers that cannot express the desired
	// result type in SQL: SQLite (and rqlite) report no usable type for a sum()
	// expression, so they register sum() here to pin it to kind.Decimal. During
	// column rendering, a match records the output position in
	// Context.ResultColumnKinds, which the driver applies when building record
	// metadata. Empty by default. See issue #839.
	FunctionResultKinds map[string]kind.Kind

	// Literal renders a literal fragment.
	Literal func(rc *Context, lit *ast.LiteralNode) (string, error)

	// Where renders a WHERE fragment.
	Where func(rc *Context, where *ast.WhereNode) (string, error)

	// Expr renders an expression fragment.
	Expr func(rc *Context, expr *ast.ExprNode) (string, error)

	// Operator renders an operator fragment.
	Operator func(rc *Context, op *ast.OperatorNode) (string, error)

	// Distinct renders the DISTINCT fragment. Returns an
	// empty string if n is nil.
	Distinct func(rc *Context, n *ast.UniqueNode) (string, error)

	// Render renders f into a SQL query.
	Render func(rc *Context, f *Fragments) (string, error)

	// PreRender is a set of hooks that are called before Render. It is a final
	// opportunity to customize f before rendering. It is nil by default.
	PreRender []func(rc *Context, f *Fragments) error
}

// NewDefaultRenderer returns a Renderer that works for most SQL dialects.
// Driver implementations can override specific rendering functions
// as needed.
func NewDefaultRenderer() *Renderer {
	return &Renderer{
		FromTable:  doFromTable,
		SelectCols: doSelectCols,
		Range:      doRange,
		OrderBy:    doOrderBy,
		GroupBy:    doGroupBy,
		Having:     doHaving,
		Join:       doJoin,
		Function:   doFunction,
		FunctionOverrides: map[string]func(rc *Context, fn *ast.FuncNode) (string, error){
			ast.FuncNameRowNum:      doFuncRowNum,
			ast.FuncNameContains:    doFuncContains,
			ast.FuncNameStartsWith:  doFuncStartsWith,
			ast.FuncNameEndsWith:    doFuncEndsWith,
			ast.FuncNameIContains:   doFuncIContains,
			ast.FuncNameIStartsWith: doFuncIStartsWith,
			ast.FuncNameIEndsWith:   doFuncIEndsWith,
			ast.FuncNameLike:        doFuncLike,
			ast.FuncNameILike:       doFuncILike,
		},
		FunctionNames:       map[string]string{},
		FunctionResultKinds: map[string]kind.Kind{},
		Literal:             doLiteral,
		Where:               doWhere,
		Expr:                doExpr,
		Operator:            doOperator,
		Distinct:            doDistinct,
		Render:              doRender,
	}
}

// Fragments holds the fragments of a SQL query.
// It is passed to Renderer.PreRender and Renderer.Render.
type Fragments struct {
	Distinct string
	Columns  string
	From     string
	Where    string
	GroupBy  string
	Having   string
	OrderBy  string
	Range    string
	// PreExecStmts are statements that are executed before the query.
	// These can be used for edge-case behavior, such as setting up
	// variables in the session.
	//
	// See also: Fragments.PostExecStmts.
	PreExecStmts []string

	// PostExecStmts are statements that are executed after the query.
	//
	// See also: Fragments.PreExecStmts.
	PostExecStmts []string
}

// doRender renders the supplied fragments into a SQL query.
func doRender(_ *Context, f *Fragments) (string, error) {
	sb := strings.Builder{}

	sb.WriteString("SELECT")

	if f.Distinct != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Distinct)
	}

	sb.WriteRune(sp)
	sb.WriteString(f.Columns)

	if f.From != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.From)
	}

	if f.Where != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Where)
	}

	if f.GroupBy != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.GroupBy)
	}

	if f.Having != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Having)
	}

	if f.OrderBy != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.OrderBy)
	}

	if f.Range != "" {
		sb.WriteRune(sp)
		sb.WriteString(f.Range)
	}

	return sb.String(), nil
}

const (
	singleQuote = '\''
	sp          = ' '
)

// renderSelectorNode renders a selector such as ".actor.first_name"
// or ".last_name".
func renderSelectorNode(d dialect.Dialect, node ast.Node) (string, error) {
	switch node := node.(type) {
	case *ast.ColSelectorNode:
		return d.Enquote(node.ColName()), nil
	case *ast.TblColSelectorNode:
		return d.Enquote(node.TblName()) + "." + d.Enquote(node.ColName()), nil
	case *ast.TblSelectorNode:
		return node.Table().Render(d.Enquote), nil
	default:
		return "", errz.Errorf(
			"expected selector node type, but got %T: %s",
			node,
			node.Text(),
		)
	}
}

// AppendSQL is a convenience function for building the SQL string.
// The main purpose is to ensure that there's always a consistent amount
// of whitespace. Thus, if existing has a space suffix and add has a
// space prefix, the returned string will only have one space. If add
// is the empty string or just whitespace, this function simply
// returns existing.
func AppendSQL(existing, add string) string {
	add = strings.TrimSpace(add)
	if add == "" {
		return existing
	}

	existing = strings.TrimSpace(existing)
	return existing + " " + add
}

// unquoteLiteral returns true if s is a double-quoted string, and also returns
// the value with the quotes stripped and JSON-style backslash escapes decoded
// (see grammar/SLQ.g4 STRING/ESC: \", \\, \/, \b, \f, \n, \r, \t, \uXXXX).
// An error is returned if the string is malformed.
func unquoteLiteral(s string) (val string, ok bool, err error) {
	hasPrefix := strings.HasPrefix(s, `"`)
	hasSuffix := strings.HasSuffix(s, `"`)

	if hasPrefix && hasSuffix {
		val, err = decodeStringLiteralBody(s[1 : len(s)-1])
		if err != nil {
			return "", false, err
		}
		return val, true, nil
	}

	if hasPrefix != hasSuffix {
		return "", false, errz.Errorf("malformed literal: %s", s)
	}

	return s, false, nil
}

// decodeStringLiteralBody decodes the body of a SLQ STRING token (the
// characters between the surrounding double quotes), translating the
// backslash escapes permitted by grammar/SLQ.g4 (\", \\, \/, \b, \f, \n,
// \r, \t, \uXXXX) into their actual rune values. Unescaped characters are
// passed through verbatim. Iteration is byte-based because all escape
// characters are ASCII; multi-byte UTF-8 sequences outside escapes pass
// through byte-for-byte unchanged.
func decodeStringLiteralBody(body string) (string, error) {
	if !strings.Contains(body, `\`) {
		return body, nil
	}
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' {
			_ = b.WriteByte(c)
			continue
		}
		i++
		if i >= len(body) {
			return "", errz.Errorf("malformed string literal: dangling backslash")
		}
		var decoded byte
		switch esc := body[i]; esc {
		case '"', '\\', '/':
			decoded = esc
		case 'b':
			decoded = '\b'
		case 'f':
			decoded = '\f'
		case 'n':
			decoded = '\n'
		case 'r':
			decoded = '\r'
		case 't':
			decoded = '\t'
		case 'u':
			if i+4 >= len(body) {
				return "", errz.Errorf(`malformed string literal: short \u escape`)
			}
			// 4 hex digits ⇒ value fits in uint16, so rune(v) is in range.
			v, err := strconv.ParseUint(body[i+1:i+5], 16, 16)
			if err != nil {
				return "", errz.Wrap(err, `malformed string literal: invalid \u escape`)
			}
			i += 4
			r := rune(v)
			if utf16.IsSurrogate(r) {
				// Combine a UTF-16 surrogate pair (e.g. 😀) into a
				// single astral codepoint, matching JSON decoding. A high
				// surrogate followed by a "\uXXXX" low surrogate yields the
				// combined rune; any unpaired surrogate decodes to U+FFFD, as
				// encoding/json does.
				if lo, ok := peekUnicodeEscape(body, i+1); ok {
					if combined := utf16.DecodeRune(r, lo); combined != unicode.ReplacementChar {
						b.WriteRune(combined)
						i += 6 // consume the trailing \uXXXX low surrogate
						continue
					}
				}
				b.WriteRune(unicode.ReplacementChar)
				continue
			}
			b.WriteRune(r)
			continue
		default:
			return "", errz.Errorf(`malformed string literal: invalid escape \%c`, esc)
		}
		_ = b.WriteByte(decoded)
	}
	return b.String(), nil
}

// peekUnicodeEscape reports whether body[i:] begins with a "\uXXXX" escape,
// returning the decoded 16-bit value as a rune when it does. It is used to
// look ahead for the low half of a UTF-16 surrogate pair.
func peekUnicodeEscape(body string, i int) (r rune, ok bool) {
	if i+6 > len(body) || body[i] != '\\' || body[i+1] != 'u' {
		return 0, false
	}
	v, err := strconv.ParseUint(body[i+2:i+6], 16, 16)
	if err != nil {
		return 0, false
	}
	return rune(v), true
}

// FuncOverrideString returns a function that always returns s.
func FuncOverrideString(s string) func(*Context, *ast.FuncNode) (string, error) {
	return func(_ *Context, _ *ast.FuncNode) (string, error) {
		return s, nil
	}
}

// AggDecimalScale is the fractional scale applied when an aggregate result
// (e.g. sum()) is cast to a fixed-scale decimal type on dialects that require
// an explicit precision and scale. It must agree across those dialects so the
// same aggregate rounds identically wherever a fixed scale is used.
//
// A sum of values with more than AggDecimalScale fractional digits is rounded
// to this scale on those dialects. Postgres uses an unconstrained NUMERIC and
// is exact, so it is not subject to this rounding. See issue #839.
const AggDecimalScale = 6

// AggDecimalPrecision is the total precision paired with AggDecimalScale on
// dialects whose decimal type caps at 38 digits (ClickHouse, Oracle, SQL
// Server). MySQL uses its higher native cap (65) instead. With scale 6 this
// leaves 32 integer digits, so a sum whose integer part exceeds that overflows
// on those dialects (a query error); Postgres (unconstrained NUMERIC) and MySQL
// (precision 65) do not. See issue #839.
const AggDecimalPrecision = 38

// FuncOverrideCastResult returns a FunctionOverrides impl that renders fn with
// the default function renderer and wraps the whole result in
// CAST(... AS castType). Use it to coerce an aggregate's result to a portable
// type where the engine's aggregate is already non-truncating, e.g.
// CAST(avg(col) AS DOUBLE PRECISION) on Postgres. See issue #594.
func FuncOverrideCastResult(castType string) func(*Context, *ast.FuncNode) (string, error) {
	return func(rc *Context, fn *ast.FuncNode) (string, error) {
		inner, err := RenderFuncDefault(rc, fn)
		if err != nil {
			return "", err
		}
		return "CAST(" + inner + " AS " + castType + ")", nil
	}
}

// FuncOverrideCastOperand returns a FunctionOverrides impl that renders fn but
// wraps each operand in CAST(... AS castType), e.g. avg(CAST(col AS FLOAT)).
// Unlike FuncOverrideCastResult, this casts the operands, which is required
// where the engine would otherwise compute the aggregate in the operand's type:
// SQL Server's AVG over an integer column performs integer division and
// truncates, so a result cast comes too late. The function name is resolved
// through Renderer.FunctionNames, matching RenderFuncDefault. See issue #594.
//
// It is intended for single-operand aggregates such as avg() and sum(). It does
// not reproduce RenderFuncDefault's count/count_unique special-casing (the
// no-arg count(*) form, or count_unique becoming count(DISTINCT ...)), so it
// must not be registered for count or count_unique.
func FuncOverrideCastOperand(castType string) func(*Context, *ast.FuncNode) (string, error) {
	return func(rc *Context, fn *ast.FuncNode) (string, error) {
		fnName := strings.ToLower(fn.FuncName())
		if mapped, ok := rc.Renderer.FunctionNames[fnName]; ok {
			fnName = mapped
		}

		children := fn.Children()
		args := make([]string, len(children))
		for i, child := range children {
			s, err := RenderFuncArg(rc, child)
			if err != nil {
				return "", err
			}
			args[i] = "CAST(" + s + " AS " + castType + ")"
		}

		return fnName + "(" + strings.Join(args, ", ") + ")", nil
	}
}
