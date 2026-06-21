// Package tablefq holds the [T] type, a fully-qualified SQL table name of
// the form CATALOG.SCHEMA.NAME. It lives in its own package so the type can
// carry a terse name ([tablefq.T]) at the many call sites that reference it:
// the driver interface ([driver.Driver.CopyTable] and friends), the AST, and
// the query pipeline all traffic in [T].
package tablefq

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"
)

// T is a fully-qualified table name, of the form CATALOG.SCHEMA.NAME,
// e.g. "sakila"."public"."actor".
//
// T is comparable: because all fields are strings, two T values can be
// compared with ==, and T can be used as a map key.
//
// The fields form a hierarchy: a [T] with a Catalog is expected to also have
// a Schema. Constructing a T with Catalog set but Schema empty is a malformed
// state; see [T.Render] for how it is handled.
type T struct {
	// Catalog is the database, e.g. "sakila".
	Catalog string

	// Schema is the namespace, e.g. "public".
	Schema string

	// Table is the table name, e.g. "actor".
	Table string
}

// String returns a default representation of t using stringz.DoubleQuote
// for the components. Use T.Render to control the output format.
func (t T) String() string {
	return t.Render(stringz.DoubleQuote)
}

// Render renders t using quoteFn, e.g. stringz.DoubleQuote. Empty components
// are omitted: a T with only Table set renders as just the quoted table name.
//
// Render assumes the hierarchy invariant (a Catalog implies a Schema). A
// malformed T with Catalog set but Schema empty renders as CATALOG.NAME,
// which most dialects would read as SCHEMA.NAME; callers must not construct
// that state.
func (t T) Render(quoteFn func(s string) string) string {
	sb := strings.Builder{}
	if t.Catalog != "" {
		sb.WriteString(quoteFn(t.Catalog))
		sb.WriteRune('.')
	}
	if t.Schema != "" {
		sb.WriteString(quoteFn(t.Schema))
		sb.WriteRune('.')
	}
	sb.WriteString(quoteFn(t.Table))
	return sb.String()
}

// New returns a new tablefq.T with the given T.Table value,
// and empty T.Catalog and T.Schema values. To construct a fully-qualified
// name, use a struct literal: tablefq.T{Catalog: c, Schema: s, Table: t}.
func New(tbl string) T {
	return T{Table: tbl}
}

// From converts t, which is either a string (just the table name) or an
// existing [T], into a [T]. It exists to unify the [Any] constraint for
// generic callers such as [Format]; for a plain string, prefer [New].
func From[X Any](t X) T {
	if tfq, ok := any(t).(T); ok {
		return tfq
	}
	// Per the Any constraint, if t is not a T it must be a string.
	return New(any(t).(string))
}

// Format renders tbl, which may be a string or a [T], using quoteFn. It is a
// convenience wrapper over [From] and [T.Render] for generic callers (e.g. a
// driver's table-name formatter that accepts either a bare name or a [T]).
func Format[X Any](tbl X, quoteFn func(s string) string) string {
	return From(tbl).Render(quoteFn)
}

// Any is a type constraint for a table name, which can be a string (just
// the table name without catalog or schema) or a fully-qualified T. It is
// used by [From] and [Format].
type Any interface {
	string | T
}
