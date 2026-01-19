package clickhouse

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// dbTypeNameFromKind maps sq kind to ClickHouse type name for CREATE TABLE.
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
		return "UInt8" // ClickHouse Bool is alias for UInt8
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
// ClickHouse requires an ENGINE and ORDER BY clause.
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

	// ORDER BY clause is required for MergeTree
	// Use first column as the ordering key by default
	if len(tblDef.Cols) > 0 {
		sb.WriteString("ORDER BY ")
		sb.WriteString(stringz.BacktickQuote(tblDef.Cols[0].Name))
	}

	return sb.String()
}

// buildUpdateStmt builds an UPDATE statement.
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
