// Package dialect contains functionality for SQL dialects.
package dialect

import "github.com/neilotoole/sq/libsq/source"

// Dialect holds driver-specific SQL dialect values and functions.
type Dialect struct {
	// Type is the dialect's driver source type.
	Type source.Type `json:"type"`

	// Placeholders returns a string a SQL placeholders string.
	// For example "(?, ?, ?)" or "($1, $2, $3), ($4, $5, $6)".
	Placeholders func(numCols, numRows int) string

	// IdentQuote is the identifier quote rune. Most often this is
	// double-quote, e.g. SELECT * FROM "my_table", but can be other
	// values such as backtick, e.g. SELECT * FROM `my_table`.
	IdentQuote rune `json:"quote"`

	// Enquote is a function that quotes and escapes an
	// identifier.
	Enquote func(string) string

	// IntBool is true if BOOLEAN is handled as an INT by the DB driver.
	IntBool bool `json:"int_bool"`

	// MaxBatchValues is the maximum number of values in a batch insert.
	MaxBatchValues int
}

// String returns a log/debug-friendly representation.
func (d Dialect) String() string {
	return d.Type.String()
}
