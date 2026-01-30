// Package dialect contains functionality for SQL dialects.
package dialect

import (
	"log/slog"
	"strings"

	udrivers "github.com/xo/usql/drivers"
	ustmt "github.com/xo/usql/stmt"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// Dialect holds driver-specific SQL dialect values and functions.
// The zero value is not usable; each driver implementation must initialize
// all fields appropriately. See the driver packages (e.g., postgres, mysql)
// for examples.
type Dialect struct {
	// Placeholders returns a string a SQL placeholders string.
	// For example "(?, ?, ?)" or "($1, $2, $3), ($4, $5, $6)".
	Placeholders func(numCols, numRows int) string

	// Enquote is a function that quotes and escapes an
	// identifier (such as a table or column name). Typically, the func
	// uses the double-quote rune (although MySQL uses backtick).
	Enquote func(string) string

	// ExecModeFor returns the ExecMode for a SQL string. The default
	// implementation is DefaultExecModeFor, which handles standard SQL.
	// This is a function pointer to allow drivers to override exec mode
	// detection if a dialect has non-standard rules (e.g., vendor-specific
	// keywords that return rows).
	ExecModeFor func(sql string) (ExecMode, error)

	// Ops is a map of SLQ operator (e.g. "==" or "!=") to its SQL rendering.
	// The default implementation is DefaultOps, which handles standard SQL
	// operators. Drivers can override specific operators if a dialect has
	// non-standard mappings.
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
// modified by the caller. This function is intended for use by driver
// initialization code; callers should use Dialect.Ops instead.
func DefaultOps() map[string]string {
	ops := make(map[string]string, len(defaultOps))
	for k, v := range defaultOps {
		ops[k] = v
	}
	return ops
}

// ExecMode indicates whether a SQL statement should be executed via DB.Query
// (returns rows) or DB.Exec (returns affected count).
//
// The naming follows Go's database/sql package conventions, where DB.Query
// returns rows and DB.Exec returns an affected count. Alternative names like
// "Statement" were considered but rejected: all SQL is technically a statement,
// and Go's *sql.Stmt specifically means a prepared statement. The usql project
// (github.com/xo/usql) uses similar "query" vs "exec" terminology.
//
// See also: https://pkg.go.dev/database/sql
type ExecMode string

const (
	// ExecModeQuery indicates a SQL query that returns rows, such as SELECT.
	// Use DB.QueryContext for this mode.
	ExecModeQuery ExecMode = "query"

	// ExecModeExec indicates a SQL statement that does not return rows, such as
	// INSERT, UPDATE, DELETE, CREATE, DROP, etc. Use DB.ExecContext for this
	// mode.
	ExecModeExec ExecMode = "exec"
)

// LogValue implements slog.LogValuer.
func (m ExecMode) LogValue() slog.Value {
	return slog.StringValue(string(m))
}

// DefaultExecModeFor returns the ExecMode for a SQL string. It uses the usql
// library (github.com/xo/usql) to determine whether the SQL should be executed
// via DB.Query (returns rows) or DB.Exec (returns affected count). An error is
// returned if the SQL string is empty, contains only whitespace/comments, or is
// invalid/unparseable such that usql's FindPrefix cannot determine a statement
// prefix. This function is intended for use by driver initialization code;
// callers should use Dialect.ExecModeFor instead.
//
// See also: https://pkg.go.dev/github.com/xo/usql/drivers#QueryExecType
func DefaultExecModeFor(sqlStr string) (ExecMode, error) {
	sqlStr = strings.TrimSpace(sqlStr)
	if sqlStr == "" {
		return "", errz.New("empty SQL string")
	}

	// Use usql's FindPrefix to extract the statement prefix (first 6 words).
	// Parameters: (sql, allowCComments, allowHashComments, allowMultilineComments)
	// Standard SQL uses -- and /* */ comments, not C-style // or hash #.
	prefix := ustmt.FindPrefix(sqlStr, false, false, true)
	if prefix == "" {
		return "", errz.New("SQL string contains only comments or is invalid")
	}

	// Use usql's QueryExecType to determine if this is a query or statement.
	// Returns (stmtType, isQuery) where isQuery=true means use Query().
	_, isQuery := udrivers.QueryExecType(prefix, sqlStr)

	if isQuery {
		return ExecModeQuery, nil
	}
	return ExecModeExec, nil
}
