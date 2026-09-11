package postgres

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
)

// tblfmt formats a table name for use in a query. The arg can be a string,
// or a tablefq.T.
func tblfmt[T string | tablefq.T](tbl T) string {
	return tablefq.Format(tbl, stringz.DoubleQuote)
}

func dbTypeNameFromKind(knd kind.Kind) string {
	switch knd { //nolint:exhaustive
	default:
		panic(fmt.Sprintf("unsupported datatype {%s}", knd))
	case kind.Unknown:
		return "TEXT"
	case kind.Text:
		return "TEXT"
	case kind.Int:
		return "BIGINT"
	case kind.Float:
		return "DOUBLE PRECISION"
	case kind.Decimal:
		return "DECIMAL"
	case kind.Bool:
		return "BOOLEAN"
	case kind.Datetime:
		return "TIMESTAMP"
	case kind.Time:
		return "TIME"
	case kind.Date:
		return "DATE"
	case kind.Bytes:
		return "BYTEA"
	}
}

// createTblKindDefaults is a map of Kind to the value
// to use for a column's DEFAULT clause in a CREATE TABLE statement.
var createTblKindDefaults = map[kind.Kind]string{ //nolint:exhaustive
	kind.Text:     `DEFAULT ''`,
	kind.Int:      `DEFAULT 0`,
	kind.Float:    `DEFAULT 0`,
	kind.Decimal:  `DEFAULT 0`,
	kind.Bool:     `DEFAULT false`,
	kind.Datetime: "DEFAULT 'epoch'::timestamp",
	kind.Date:     "DEFAULT 'epoch'::date",
	kind.Time:     "DEFAULT '00:00:00'::time",
	kind.Bytes:    "DEFAULT ''::bytea",
	kind.Unknown:  `DEFAULT ''`,
}

// buildCreateTableStmt builds a CREATE TABLE statement from tblDef.
// The implementation is minimal: it honors PKColName (as an inline
// PRIMARY KEY, which implies NOT NULL) but not FK, etc. The PRIMARY KEY
// clause is emitted after the DEFAULT / NOT NULL block for cross-driver
// consistency: Oracle requires DEFAULT before inline constraints, and the
// sqlserver and oracle builders mirror this function (#1029).
func buildCreateTableStmt(tblDef *schema.Table) string {
	sb := strings.Builder{}
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(idSanitize(tblDef.Name))
	sb.WriteString(" (")

	for i, colDef := range tblDef.Cols {
		sb.WriteString("\n")
		sb.WriteString(idSanitize(colDef.Name))
		sb.WriteString(" ")
		sb.WriteString(dbTypeNameFromKind(colDef.Kind))

		if colDef.NotNull {
			sb.WriteRune(' ')
			sb.WriteString(createTblKindDefaults[colDef.Kind])
			sb.WriteString(" NOT NULL")
		}

		if colDef.Name == tblDef.PKColName {
			sb.WriteString(" PRIMARY KEY")
		}

		if i < len(tblDef.Cols)-1 {
			sb.WriteRune(',')
		}
	}
	sb.WriteString("\n)")

	return sb.String()
}

// buildUpdateStmt builds an UPDATE statement string.
func buildUpdateStmt(tbl string, cols []string, where string) (string, error) {
	if len(cols) == 0 {
		return "", errz.Errorf("no columns provided")
	}

	sb := strings.Builder{}
	sb.WriteString("UPDATE ")
	sb.WriteString(idSanitize(tbl))
	sb.WriteString(" SET ")
	for i, col := range cols {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(idSanitize(col))
		sb.WriteString(" = ?")
	}
	if where != "" {
		sb.WriteString(" WHERE ")
		sb.WriteString(where)
	}

	s := replacePlaceholders(sb.String())
	return s, nil
}

// replacePlaceholders replaces all instances of the question mark
// rune in input with $1, $2, $3 placeholders.
func replacePlaceholders(input string) string {
	if input == "" {
		return input
	}

	var sb strings.Builder
	pCount := 1
	var i int
	for {
		i = strings.IndexRune(input, '?')
		if i == -1 {
			sb.WriteString(input)
			break
		}
		// Found a ?
		sb.WriteString(input[0:i])
		sb.WriteRune('$')
		sb.WriteString(strconv.Itoa(pCount))
		pCount++
		if i == len(input)-1 {
			break
		}
		input = input[i+1:]
	}

	return sb.String()
}
