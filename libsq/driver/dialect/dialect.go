// Package dialect contains functionality for SQL dialects.
package dialect

import (
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/source"
)

// Dialect holds driver-specific SQL dialect values and functions.
type Dialect struct {
	// Type is the dialect's driver type.
	Type source.DriverType `json:"type"`

	// Placeholders returns a string a SQL placeholders string.
	// For example "(?, ?, ?)" or "($1, $2, $3), ($4, $5, $6)".
	Placeholders func(numCols, numRows int) string

	// IdentQuote is the identifier quote rune. Most often this is
	// double-quote, e.g. SELECT * FROM "my_table", but can be other
	// values such as backtick, e.g. SELECT * FROM `my_table`.
	//
	// Arguably, this field should be deprecated. There's probably
	// no reason not to always use Enquote.
	//
	// Deprecated: Use Enquote instead.
	IdentQuote rune `json:"quote"`

	// Enquote is a function that quotes and escapes an
	// identifier (such as a table or column name).
	Enquote func(string) string

	// IntBool is true if BOOLEAN is handled as an INT by the DB driver.
	IntBool bool `json:"int_bool"`

	// MaxBatchValues is the maximum number of values in a batch insert.
	MaxBatchValues int

	// Ops is a map of overridden SLQ operator (e.g. "==" or "!=") to
	// its default SQL rendering.
	//
	// Deprecated: Ops doesn't need to exist.
	Ops map[string]string

	// Joins is the set of JOIN types (e.g. "RIGHT JOIN") that
	// the dialect supports. Not all drivers support each join type. For
	// example, MySQL doesn't support jointype.FullOuter.
	Joins []jointype.Type
}

// String returns a log/debug-friendly representation.
func (d Dialect) String() string {
	return d.Type.String()
}

// defaultOps is a map of SLQ operator (e.g. "==" or "!=") to
// its default SQL rendering.
var defaultOps = map[string]string{
	`==`: `=`,
	`&&`: `AND`,
	`||`: `OR`,
	`!=`: `!=`,
}

// DefaultOps returns a default map of SLQ operator (e.g. "==" or "!=") to
// its SQL rendering. The returned map is a copy and can be safely
// modified by the caller.
func DefaultOps() map[string]string {
	ops := make(map[string]string, len(defaultOps))
	for k, v := range defaultOps {
		ops[k] = v
	}
	return ops
}
