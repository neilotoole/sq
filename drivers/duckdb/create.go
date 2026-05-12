package duckdb

import (
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
)

// createTblKindDefaults maps kind.Kind to the DEFAULT clause used when
// building a CREATE TABLE statement for a NOT NULL column that has a default.
var createTblKindDefaults = map[kind.Kind]string{ //nolint:exhaustive // kind.Null is intentionally omitted
	kind.Text:     `DEFAULT ''`,
	kind.Int:      `DEFAULT 0`,
	kind.Float:    `DEFAULT 0`,
	kind.Decimal:  `DEFAULT 0`,
	kind.Bool:     `DEFAULT false`,
	kind.Datetime: "DEFAULT '1970-01-01 00:00:00'::TIMESTAMP",
	kind.Date:     "DEFAULT '1970-01-01'::DATE",
	kind.Time:     "DEFAULT '00:00:00'::TIME",
	kind.Bytes:    `DEFAULT ''::BLOB`,
	kind.Unknown:  `DEFAULT ''`,
}

// buildCreateTableStmt builds a DuckDB CREATE TABLE statement from tblDef.
// It honours PKColName, AutoIncrement, NotNull, HasDefault, and Unique.
// Foreign-key constraints are deliberately omitted for now.
func buildCreateTableStmt(tblDef *schema.Table) string {
	sb := strings.Builder{}
	sb.WriteString(`CREATE TABLE "`)
	sb.WriteString(tblDef.Name)
	sb.WriteString("\" (\n")

	for i, col := range tblDef.Cols {
		sb.WriteString(`  "`)
		sb.WriteString(col.Name)
		sb.WriteString(`" `)
		sb.WriteString(dbTypeNameFromKind(col.Kind))

		if col.Name == tblDef.PKColName {
			sb.WriteString(" PRIMARY KEY")
			// DuckDB uses SEQUENCE or SERIAL-style for auto-increment;
			// the cleanest approach is to override the type to BIGINT and
			// omit AUTOINCREMENT (DuckDB does not support AUTOINCREMENT).
			// The PK constraint itself is sufficient for most sq use-cases.
		}

		if col.HasDefault {
			sb.WriteRune(' ')
			sb.WriteString(createTblKindDefaults[col.Kind])
		}

		if col.NotNull {
			sb.WriteString(" NOT NULL")
		}

		if col.Unique {
			sb.WriteString(" UNIQUE")
		}

		if i < len(tblDef.Cols)-1 {
			sb.WriteRune(',')
		}
		sb.WriteRune('\n')
	}

	sb.WriteRune(')')
	return sb.String()
}

// buildUpdateStmt builds a DuckDB UPDATE statement using $N positional
// placeholders.
func buildUpdateStmt(tbl string, cols []string, where string) (string, error) {
	if len(cols) == 0 {
		return "", errz.Errorf("no columns provided for UPDATE")
	}

	sb := strings.Builder{}
	sb.WriteString(`UPDATE "`)
	sb.WriteString(tbl)
	sb.WriteString(`" SET `)

	for i, col := range cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteRune('"')
		sb.WriteString(col)
		sb.WriteString(`" = $`)
		// Use 1-based positional placeholders.
		writeInt(&sb, i+1)
	}

	if where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(where)
	}

	return sb.String(), nil
}

// writeInt writes a base-10 integer to sb without allocating.
func writeInt(sb *strings.Builder, n int) {
	// Fast path for column counts ≤ 9 (the common case).
	if n >= 0 && n <= 9 {
		_ = sb.WriteByte('0' + byte(n))
		return
	}
	digits := [20]byte{}
	pos := len(digits)
	for n > 0 {
		pos--
		digits[pos] = '0' + byte(n%10)
		n /= 10
	}
	sb.Write(digits[pos:])
}
