// Package tablefq is a tiny package that holds the tablefq.T type, which
// is a fully-qualified SQL table name. This package is an experiment,
// and may be changed or removed.
package tablefq

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"
)

// T is a fully-qualified table name, of the form CATALOG.SCHEMA.NAME,
// e.g. "sakila"."public"."actor".
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

// Render renders t using escapeFn, e.g. stringz.DoubleQuote.
func (t T) Render(escapeFn func(s string) string) string {
	sb := strings.Builder{}
	if t.Catalog != "" {
		sb.WriteString(escapeFn(t.Catalog))
		sb.WriteRune('.')
	}
	if t.Schema != "" {
		sb.WriteString(escapeFn(t.Schema))
		sb.WriteRune('.')
	}
	sb.WriteString(escapeFn(t.Table))
	return sb.String()
}

func New(name string) T {
	return T{Table: name}
}

// From returns a T from a Table, which can be either
// a string (just the table name) or a fully-qualified table,
// including catalog, schema, and table name.
func From[X Table](t X) T {
	if tfq, ok := any(t).(T); ok {
		return tfq
	}

	if s, ok := any(t).(string); ok {
		return T{Table: s}
	}

	panic(fmt.Sprintf("invalid type %T: %v", t, t))
}

// Table is a type constraint for a table name, which can be a string (just
// the table name without catalog or schema) or a fully-qualified T.
type Table interface {
	string | T
}
