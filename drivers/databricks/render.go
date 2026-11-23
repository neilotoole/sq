package databricks

import (
	"fmt"
	"math"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
)

// tblfmt renders a table reference as a backtick-quoted identifier, keeping
// any catalog and schema qualifiers. It accepts either a plain table name or
// a [tablefq.T].
//
// Examples:
//
//	tblfmt("actor")                                    -> `actor`
//	tblfmt(tablefq.T{Schema: "sakila", Table: "t"})    -> `sakila`.`t`
func tblfmt[T string | tablefq.T](tbl T) string {
	return tablefq.Format(tbl, stringz.BacktickQuote)
}

// quoteString returns s as a Spark SQL string literal.
//
// Spark SQL treats backslash as an escape character in string literals, and
// it does not read a doubled single quote as a quote. It reads the pair as
// the end of one literal and the start of the next, and concatenates the two,
// so the value it's would become its. The usual doubling done by
// stringz.SingleQuote is therefore wrong here, and for a value containing a
// backslash it can also end the literal early. Instead, both backslash and
// single quote are escaped with a backslash.
//
//	O'Brien  -->  'O\'Brien'
//	a\b      -->  'a\\b'
func quoteString(s string) string {
	var sb strings.Builder
	sb.Grow(len(s) + 2)
	sb.WriteByte('\'')
	for _, r := range s {
		if r == '\'' || r == '\\' {
			sb.WriteByte('\\')
		}
		sb.WriteRune(r)
	}
	sb.WriteByte('\'')
	return sb.String()
}

// renderRange renders the row range (LIMIT and OFFSET) fragment.
//
// This differs from the default renderer in two ways. Spark requires the
// LIMIT expression to be an INT, so a limit above [math.MaxInt32] (which is
// no limit in practice) is omitted rather than rendered as a BIGINT literal.
// And Databricks SQL accepts OFFSET without LIMIT, so the default's trick of
// supplying a huge LIMIT is not needed (and would not be valid).
func renderRange(_ *render.Context, rr *ast.RowRangeNode) (string, error) {
	if rr == nil {
		return "", nil
	}

	var limit, offset string
	if rr.Limit > -1 && rr.Limit <= math.MaxInt32 {
		limit = fmt.Sprintf("LIMIT %d", rr.Limit)
	}
	if rr.Offset > -1 {
		offset = fmt.Sprintf("OFFSET %d", rr.Offset)
	}

	return strings.TrimSpace(render.AppendSQL(limit, offset)), nil
}
