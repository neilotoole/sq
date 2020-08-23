package postgres

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/sqlbuilder"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/sqlmodel"
)

func newFragmentBuilder(log lg.Log) *sqlbuilder.BaseFragmentBuilder {
	fb := &sqlbuilder.BaseFragmentBuilder{}
	fb.Log = log
	fb.Quote = `"`
	fb.ColQuote = `"`
	fb.Ops = sqlbuilder.BaseOps()
	return fb
}

func dbTypeNameFromKind(kind sqlz.Kind) string {
	switch kind {
	default:
		panic(fmt.Sprintf("unsupported datatype %q", kind))
	case sqlz.KindUnknown:
		return "TEXT"
	case sqlz.KindText:
		return "TEXT"
	case sqlz.KindInt:
		return "BIGINT"
	case sqlz.KindFloat:
		return "DOUBLE PRECISION"
	case sqlz.KindDecimal:
		return "DECIMAL"
	case sqlz.KindBool:
		return "BOOLEAN"
	case sqlz.KindDatetime:
		return "TIMESTAMP"
	case sqlz.KindTime:
		return "TIME"
	case sqlz.KindDate:
		return "DATE"
	case sqlz.KindBytes:
		return "BYTEA"
	}
}

// createTblKindDefaults is a map of Kind to the value
// to use for a column's DEFAULT clause in a CREATE TABLE statement.
var createTblKindDefaults = map[sqlz.Kind]string{
	sqlz.KindText:     `DEFAULT ''`,
	sqlz.KindInt:      `DEFAULT 0`,
	sqlz.KindFloat:    `DEFAULT 0`,
	sqlz.KindDecimal:  `DEFAULT 0`,
	sqlz.KindBool:     `DEFAULT false`,
	sqlz.KindDatetime: "DEFAULT 'epoch'::timestamp",
	sqlz.KindDate:     "DEFAULT 'epoch'::date",
	sqlz.KindTime:     "DEFAULT '00:00:00'::time",
	sqlz.KindBytes:    "DEFAULT ''::bytea",
	sqlz.KindUnknown:  `DEFAULT ''`,
}

// buildCreateTableStmt builds a CREATE TABLE statement from tblDef.
// The implementation is minimal: it does not honor PK, FK, etc.
func buildCreateTableStmt(tblDef *sqlmodel.TableDef) string {
	sb := strings.Builder{}
	sb.WriteString(`CREATE TABLE "`)
	sb.WriteString(tblDef.Name)
	sb.WriteString("\" (")

	for i, colDef := range tblDef.Cols {
		sb.WriteString("\n\"")
		sb.WriteString(colDef.Name)
		sb.WriteString("\" ")
		sb.WriteString(dbTypeNameFromKind(colDef.Kind))

		if colDef.NotNull {
			sb.WriteRune(' ')
			sb.WriteString(createTblKindDefaults[colDef.Kind])
			sb.WriteString(" NOT NULL")
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
	sb.WriteString(`UPDATE "`)
	sb.WriteString(tbl)
	sb.WriteString(`" SET "`)
	sb.WriteString(strings.Join(cols, `" = ?, "`))
	sb.WriteString(`" = ?`)
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
		} else {
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
	}

	return sb.String()
}
