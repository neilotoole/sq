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

const (
	// TypeSL3 is the sqlite3 source driver type.
	TypeSL3 Type = "sqlite3"

	// TypeMS is the SQL Server source driver type.
	TypeMS = Type("sqlserver")

	// TypeCSV is the CSV driver type.
	TypeCSV = Type("csv")

	// TypeTSV is the TSV driver type.
	TypeTSV = Type("tsv")

	// TypeJSON is the plain-old JSON driver type.
	TypeJSON = Type("json")

	// TypeJSONA is the JSON Array driver type.
	TypeJSONA = Type("jsona")

	// TypeJSONL is the JSON Lines driver type.
	TypeJSONL = Type("jsonl")
	// TypeMy is the MySQL source driver type.
	TypeMy = Type("mysql")

	// TypePg is the postgres source driver type.
	TypePg = Type("postgres")

	// TypeXLSX is the sq source driver type for XLSX.
	TypeXLSX = Type("xlsx")
)
