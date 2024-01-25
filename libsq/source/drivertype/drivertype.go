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
	TypeSL3  = Type("sqlite3")
	TypePg   = Type("postgres")
	TypeMS   = Type("sqlserver")
	TypeMy   = Type("mysql")
	TypeXLSX = Type("xlsx")
	TypeCSV  = Type("csv")
	TypeTSV  = Type("tsv")
)
