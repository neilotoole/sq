// Package dialect contains functionality for SQL dialects.
package dialect

import (
	"log/slog"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// Dialect holds driver-specific SQL dialect values and functions.
// The zero value is not usable; each driver implementation must initialize
// all fields appropriately. See the driver packages (e.g., postgres, mysql)
// for examples.
type Dialect struct {
	// TODO: Consider adding a field to indicate whether the dialect can report
	// rows affected for bulk operations like INSERT ... SELECT. Some databases
	// (e.g., ClickHouse) cannot report row counts for these operations, and
	// currently this is handled by returning RowsAffectedUnavailable (-1) from
	// methods like SQLDriver.CopyTable. A dialect field like "CanReportBulkRowsAffected"
	// would allow callers to check upfront rather than handling the -1 case.

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

// RowsAffectedUnavailable is a sentinel value (-1) returned by operations like
// [driver.SQLDriver.CopyTable] when the number of affected rows cannot be
// determined. Some databases (e.g., ClickHouse) do not report row counts for
// certain operations like INSERT ... SELECT.
//
// Callers should check for this value before using the row count:
//
//	copied, err := drvr.CopyTable(ctx, db, from, to, true)
//	if err != nil {
//	    return err
//	}
//	if copied == dialect.RowsAffectedUnavailable {
//	    // Row count unavailable; verify success via other means if needed
//	} else {
//	    fmt.Printf("Copied %d rows\n", copied)
//	}
//
// This follows a common pattern where -1 indicates "unknown" (similar to
// HTTP Content-Length: -1 for chunked encoding).
const RowsAffectedUnavailable int64 = -1

// LogValue implements slog.LogValuer.
func (m ExecMode) LogValue() slog.Value {
	return slog.StringValue(string(m))
}

// DefaultExecModeFor returns the ExecMode for a SQL string. It returns
// ExecModeQuery if the SQL appears to be a query (SELECT, WITH, SHOW, etc.)
// that returns rows, or ExecModeExec if it's a statement (CREATE, INSERT,
// UPDATE, etc.) that should use ExecContext. An error is returned if the SQL
// string is empty or contains only whitespace/comments. This function is
// intended for use by driver initialization code; callers should use
// Dialect.ExecModeFor instead.
func DefaultExecModeFor(sqlStr string) (ExecMode, error) {
	query, err := isQueryString(sqlStr)
	if err != nil {
		return "", err
	}
	if query {
		return ExecModeQuery, nil
	}
	return ExecModeExec, nil
}

// isQueryString returns true if the SQL appears to be a query (SELECT, WITH,
// SHOW, etc.) that returns rows, false if it's a statement (CREATE, INSERT,
// UPDATE, etc.) that should use ExecContext. An error is returned if the SQL
// string is empty or contains only whitespace/comments.
func isQueryString(sqlStr string) (bool, error) {
	sqlStr = strings.TrimSpace(sqlStr)
	if sqlStr == "" {
		return false, errz.New("empty SQL string")
	}

	// Remove leading comments
	for strings.HasPrefix(sqlStr, "--") || strings.HasPrefix(sqlStr, "/*") {
		if strings.HasPrefix(sqlStr, "--") {
			idx := strings.Index(sqlStr, "\n")
			if idx < 0 {
				// Only a line comment, no actual SQL
				return false, errz.New("SQL string contains only comments")
			}
			sqlStr = strings.TrimSpace(sqlStr[idx+1:])
		} else if strings.HasPrefix(sqlStr, "/*") {
			idx := strings.Index(sqlStr, "*/")
			if idx < 0 {
				return false, errz.New("SQL string contains unclosed block comment")
			}
			sqlStr = strings.TrimSpace(sqlStr[idx+2:])
		}
	}

	// After stripping comments, check if there's any SQL left
	if sqlStr == "" {
		return false, errz.New("SQL string contains only comments")
	}

	sqlUpper := strings.ToUpper(sqlStr)

	// Check for query statements (return rows)
	queryPrefixes := []string{"SELECT", "WITH", "SHOW", "DESCRIBE", "DESC", "EXPLAIN"}
	for _, prefix := range queryPrefixes {
		if strings.HasPrefix(sqlUpper, prefix+" ") || strings.HasPrefix(sqlUpper, prefix+"\t") ||
			strings.HasPrefix(sqlUpper, prefix+"\n") || strings.HasPrefix(sqlUpper, prefix+"\r") ||
			sqlUpper == prefix {
			return true, nil
		}
	}

	// Everything else (CREATE, INSERT, UPDATE, DELETE, DROP, ALTER, etc.) is a statement
	return false, nil
}
