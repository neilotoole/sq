package clickhouse

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
)

// tblfmt renders a table reference as a backtick-quoted identifier
// suitable for use in ClickHouse SQL statements. It accepts either
// a plain string table name or a [tablefq.T] fully-qualified table
// reference. When the input includes schema or catalog qualifiers,
// they are preserved in the output (e.g. "`mydb`.`mytable`").
//
// ClickHouse uses backtick quoting for identifiers, the same
// convention as MySQL. The [stringz.BacktickQuote] function escapes
// any embedded backticks by doubling them.
//
// This helper mirrors the tblfmt functions in the postgres, mysql,
// and sqlserver drivers, ensuring that DDL/DML operations such as
// [driveri.DropTable] and [driveri.CopyTable] correctly render
// schema-qualified table names rather than silently discarding
// the catalog/schema components.
//
// Examples:
//
//	tblfmt("actors")                              -> "`actors`"
//	tblfmt(tablefq.T{Table: "actors"})            -> "`actors`"
//	tblfmt(tablefq.T{Schema: "db", Table: "t"})   -> "`db`.`t`"
func tblfmt[T string | tablefq.T](tbl T) string {
	tfq := tablefq.From(tbl)
	return tfq.Render(stringz.BacktickQuote)
}

// dbTypeNameFromKind maps sq kind.Kind values to ClickHouse type names for use
// in CREATE TABLE statements and other DDL operations.
//
// Type mapping:
//
//	sq Kind      -> ClickHouse Type
//	--------------------------------
//	kind.Unknown -> String
//	kind.Null    -> String
//	kind.Text    -> String
//	kind.Int     -> Int64 (64-bit signed integer)
//	kind.Float   -> Float64 (double precision)
//	kind.Decimal -> Decimal(18,4) (18 digits, 4 decimal places)
//	kind.Bool    -> Bool (native Bool type, available since ClickHouse 21.12)
//	kind.Datetime-> DateTime
//	kind.Date    -> Date
//	kind.Time    -> DateTime (ClickHouse has no separate time-only type)
//	kind.Bytes   -> String (binary data stored as String)
//
// This is the inverse of kindFromClickHouseType in metadata.go, though not
// a perfect round-trip since multiple ClickHouse types map to single sq kinds.
func dbTypeNameFromKind(knd kind.Kind) string {
	switch knd {
	case kind.Unknown, kind.Null, kind.Text:
		return "String"
	case kind.Int:
		return "Int64"
	case kind.Float:
		return "Float64"
	case kind.Decimal:
		return "Decimal(18,4)"
	case kind.Bool:
		return "Bool"
	case kind.Datetime:
		return "DateTime"
	case kind.Date:
		return "Date"
	case kind.Time:
		return "DateTime"
	case kind.Bytes:
		return "String" // Binary data as String
	}
	return "String"
}

// buildCreateTableStmt builds a CREATE TABLE statement for ClickHouse.
//
// ClickHouse tables differ from traditional SQL tables in several ways:
//
//  1. ENGINE clause is required. This function uses MergeTree(), ClickHouse's
//     most common engine for OLAP workloads. MergeTree provides efficient
//     data storage, compression, and query performance.
//
//  2. ORDER BY clause is required for MergeTree. This defines the primary
//     sort order for data storage and affects query performance. This function
//     uses the first NOT NULL column as the ordering key. If no NOT NULL column
//     exists, it uses tuple() which means no specific ordering.
//
//  3. Nullable types must be explicit. Unlike many SQL databases where columns
//     are nullable by default, ClickHouse columns are non-nullable by default.
//     This function wraps types with Nullable(T) when colDef.NotNull is false.
//
// Generated SQL format:
//
//	CREATE TABLE `table_name` (
//	  `col1` Type1,
//	  `col2` Nullable(Type2),
//	  ...
//	) ENGINE = MergeTree()
//	ORDER BY `col1`  -- or ORDER BY tuple() if all columns are nullable
func buildCreateTableStmt(tblDef *schema.Table) string {
	sb := strings.Builder{}
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(stringz.BacktickQuote(tblDef.Name))
	sb.WriteString(" (\n")

	for i, colDef := range tblDef.Cols {
		sb.WriteString("  ")
		sb.WriteString(stringz.BacktickQuote(colDef.Name))
		sb.WriteString(" ")

		typeName := dbTypeNameFromKind(colDef.Kind)
		if !colDef.NotNull {
			// Wrap with Nullable for columns that allow NULL values
			typeName = "Nullable(" + typeName + ")"
		}
		sb.WriteString(typeName)

		if i < len(tblDef.Cols)-1 {
			sb.WriteString(",\n")
		}
	}

	sb.WriteString("\n) ENGINE = MergeTree()\n")

	// ORDER BY clause is required for MergeTree.
	// ClickHouse does not allow nullable columns in the sorting key by default.
	// Find the first NOT NULL column to use as the ordering key, or use tuple()
	// if all columns are nullable (tuple() means no specific ordering).
	sb.WriteString("ORDER BY ")
	orderByCol := ""
	for _, colDef := range tblDef.Cols {
		if colDef.NotNull {
			orderByCol = colDef.Name
			break
		}
	}
	if orderByCol != "" {
		sb.WriteString(stringz.BacktickQuote(orderByCol))
	} else {
		sb.WriteString("tuple()")
	}

	return sb.String()
}

// buildUpdateStmt builds an UPDATE statement using ClickHouse's ALTER TABLE
// UPDATE syntax. It returns an error if cols is empty, matching the
// behavior of the equivalent function in the postgres, mysql, sqlite3,
// and sqlserver drivers.
//
// ClickHouse does not support standard SQL UPDATE statements. Instead,
// row-level updates are performed using ALTER TABLE ... UPDATE, which
// is an asynchronous mutation operation. These updates:
//
//   - Are processed in the background by ClickHouse.
//   - May take time to complete depending on data volume.
//   - Are eventually consistent (not immediately visible).
//   - Cannot be rolled back once started.
//
// The caller ([driveri.PrepareUpdateStmt]) appends
// "SETTINGS mutations_sync = 1" to force synchronous execution.
//
// A WHERE clause is always emitted. When the where parameter is empty,
// the literal "1" (always-true) is used because ClickHouse requires
// a WHERE clause in ALTER TABLE ... UPDATE. Without it, any appended
// SETTINGS clause would land in an invalid syntactic position.
//
// Generated SQL format:
//
//	ALTER TABLE `tbl` UPDATE `col1` = ?, `col2` = ? WHERE <condition>
//
// Example outputs:
//
//	buildUpdateStmt("actors", ["name"], "id = 1")
//	  -> "ALTER TABLE `actors` UPDATE `name` = ? WHERE id = 1"
//
//	buildUpdateStmt("actors", ["name", "age"], "")
//	  -> "ALTER TABLE `actors` UPDATE `name` = ?, `age` = ? WHERE 1"
//
// Parameters:
//   - tbl: table name to update (backtick-quoted in the output).
//   - cols: column names to set; must be non-empty. Each column gets
//     a positional "?" placeholder for the value.
//   - where: WHERE clause body without the "WHERE" keyword. Pass ""
//     to update all rows (emits "WHERE 1").
func buildUpdateStmt(tbl string, cols []string, where string) (string, error) {
	if len(cols) == 0 {
		return "", errz.Errorf("no columns provided")
	}

	sb := strings.Builder{}
	sb.WriteString("ALTER TABLE ")
	sb.WriteString(stringz.BacktickQuote(tbl))
	sb.WriteString(" UPDATE ")

	for i, col := range cols {
		sb.WriteString(stringz.BacktickQuote(col))
		sb.WriteString(" = ?")
		if i < len(cols)-1 {
			sb.WriteString(", ")
		}
	}

	sb.WriteString(" WHERE ")
	if where != "" {
		sb.WriteString(where)
	} else {
		// ClickHouse requires a WHERE clause in ALTER TABLE ... UPDATE.
		// Use "1" (always-true) to update all rows.
		sb.WriteString("1")
	}

	return sb.String(), nil
}
