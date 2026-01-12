// Package drivertype defines drivertype.Type, which is the type of a driver,
// e.g. "mysql", "postgres", "csv", etc. This is broken out into its own
// package to avoid circular dependencies.
package drivertype

// Type is a driver type, e.g. "mysql", "postgres", "csv", etc.
type Type string

// String returns a log/debug-friendly representation.
func (t Type) String() string {
	return string(t)
}

// None is the zero value of Type.
const None = Type("")

// Driver types.
const (
	// SQLite is for sqlite3.
	SQLite = Type("sqlite3")

	// Pg is for Postgres.
	Pg = Type("postgres")

	// MSSQL is for Microsoft SQL Server.
	MSSQL = Type("sqlserver")

	// MySQL is for MySQL and similar DBs such as MariaDB.
	MySQL = Type("mysql")

	// Oracle is for Oracle Database.
	Oracle = Type("oracle")

	// CSV is for Comma-Separated Values.
	CSV = Type("csv")

	// TSV is for Tab-Separated Values.
	TSV = Type("tsv")

	// JSON is for plain-old JSON.
	JSON = Type("json")

	// JSONA is for JSON Array.
	JSONA = Type("jsona")

	// JSONL is for JSON Lines, aka ndjson (newline-delimited).
	JSONL = Type("jsonl")

	// XLSX is for Microsoft Excel spreadsheets.
	XLSX = Type("xlsx")
)
