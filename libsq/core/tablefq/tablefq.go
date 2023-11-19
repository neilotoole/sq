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

// Render renders t using quoteFn, e.g. stringz.DoubleQuote.
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
// and empty T.Catalog and T.Schema values.
func New(tbl string) T {
	return T{Table: tbl}
}

// From returns a T from an Any, which can be either
// a string (just the table name) or a fully-qualified table,
// including catalog, schema, and table name.
func From[X Any](t X) T {
	if tfq, ok := any(t).(T); ok {
		return tfq
	}

	if s, ok := any(t).(string); ok {
		return T{Table: s}
	}

	panic(fmt.Sprintf("invalid type %T: %v", t, t))
}

// Any is a type constraint for a table name, which can be a string (just
// the table name without catalog or schema) or a fully-qualified T.
//
// REVISIT: Probably get rid of tablefq.Any, if it's only used in one place.
type Any interface {
	string | T
}
