// Package dialect contains functionality for SQL dialects.
package dialect

import (
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// Dialect holds driver-specific SQL dialect values and functions.
type Dialect struct {
	// Placeholders returns a string a SQL placeholders string.
	// For example "(?, ?, ?)" or "($1, $2, $3), ($4, $5, $6)".
	Placeholders func(numCols, numRows int) string

	// Enquote is a function that quotes and escapes an
	// identifier (such as a table or column name). Typically the func
	// uses the double-quote rune (although MySQL uses backtick).
	Enquote func(string) string

	// Ops is a map of overridden SLQ operator (e.g. "==" or "!=") to
	// its SQL rendering.
	Ops map[string]string

	// Type is the dialect's driver type.
	Type drivertype.Type `json:"type"`

	// Joins is the set of JOIN types (e.g. "RIGHT JOIN") that
	// the dialect supports. Not all drivers support each join type. For
	// example, MySQL doesn't support jointype.FullOuter.
	Joins []jointype.Type

	// MaxBatchValues is the maximum number of values in a batch insert.
	MaxBatchValues int

	// IntBool is true if BOOLEAN is handled as an INT by the DB driver.
	IntBool bool `json:"int_bool"`

	// Catalog indicates that the database supports the catalog concept,
	// in addition to schema. For example, PostgreSQL supports catalog
	// and schema (sakila.public.actor), whereas MySQL only supports schema
	// (sakila.actor).
	Catalog bool
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
