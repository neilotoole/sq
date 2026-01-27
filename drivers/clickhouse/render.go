package clickhouse

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

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
// UPDATE syntax.
//
// ClickHouse does not support standard SQL UPDATE statements. Instead, row-level
// updates are performed using ALTER TABLE ... UPDATE, which is an asynchronous
// mutation operation. These updates:
//
//   - Are processed in the background by ClickHouse
//   - May take time to complete depending on data volume
//   - Are eventually consistent (not immediately visible)
//   - Cannot be rolled back once started
//
// Generated SQL format:
//
//	ALTER TABLE `table_name` UPDATE `col1` = ?, `col2` = ? [WHERE condition]
//
// Parameters:
//   - tbl: Table name to update
//   - cols: Column names to set (values provided via ? placeholders)
//   - where: WHERE clause without the "WHERE" keyword (empty for no filter)
func buildUpdateStmt(tbl string, cols []string, where string) string {
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

	if where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(where)
	}

	return sb.String()
}
