package sqlserver

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/ast/sqlbuilder"
	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
)

var _ sqlbuilder.FragmentBuilder = (*fragBuilder)(nil)

type fragBuilder struct {
	sqlbuilder.BaseFragmentBuilder
}

func newFragmentBuilder(log lg.Log) *fragBuilder {
	r := &fragBuilder{}
	r.Log = log
	r.Quote = `"`
	r.ColQuote = `"`
	r.Ops = sqlbuilder.BaseOps()
	return r
}

func (fb *fragBuilder) Range(rr *ast.RowRangeNode) (string, error) {
	if rr == nil {
		return "", nil
	}

	/*
		SELECT * FROM actor
			ORDER BY (SELECT 0)
			OFFSET 1 ROWS
			FETCH NEXT 2 ROWS ONLY;
	*/

	if rr.Limit < 0 && rr.Offset < 0 {
		return "", nil
	}

	offset := 0
	if rr.Offset > 0 {
		offset = rr.Offset
	}

	buf := &bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("OFFSET %d ROWS", offset))

	if rr.Limit > -1 {
		buf.WriteString(fmt.Sprintf(" FETCH NEXT %d ROWS ONLY", rr.Limit))
	}

	sql := buf.String()
	fb.Log.Debugf("returning SQL fragment: %s", sql)
	return sql, nil
}

var _ sqlbuilder.QueryBuilder = (*queryBuilder)(nil)

type queryBuilder struct {
	sqlbuilder.BaseQueryBuilder
}

func (qb *queryBuilder) SQL() (string, error) {
	// SQL Server handles range (OFFSET, LIMIT) a little differently. If the query has a range,
	// then the ORDER BY clause is required. If ORDER BY is not specified, we use a trick (SELECT 0)
	// to satisfy SQL Server. For example:
	//
	//   SELECT * FROM actor
	//   ORDER BY (SELECT 0)
	//   OFFSET 1 ROWS
	//   FETCH NEXT 2 ROWS ONLY;
	if qb.RangeClause != "" {
		if qb.OrderByClause == "" {
			qb.OrderByClause = "ORDER BY (SELECT 0)"
		}
	}

	return qb.BaseQueryBuilder.SQL()
}

func dbTypeNameFromKind(knd kind.Kind) string {
	switch knd { //nolint:exhaustive // ignore kind.Null
	default:
		panic(fmt.Sprintf("unsupported datatype %q", knd))
	case kind.Unknown:
		return "NVARCHAR(MAX)"
	case kind.Text:
		return "NVARCHAR(MAX)"
	case kind.Int:
		return "BIGINT"
	case kind.Float:
		return "FLOAT"
	case kind.Decimal:
		return "DECIMAL"
	case kind.Bool:
		return "BIT"
	case kind.Datetime:
		return "DATETIME"
	case kind.Time:
		return "TIME"
	case kind.Date:
		return "DATE"
	case kind.Bytes:
		return "VARBINARY(MAX)"
	}
}

// createTblKindDefaults is a map of Kind to the value
// to use for a column's DEFAULT clause in a CREATE TABLE statement.
var createTblKindDefaults = map[kind.Kind]string{ //nolint:exhaustive
	kind.Text:     `DEFAULT ''`,
	kind.Int:      `DEFAULT 0`,
	kind.Float:    `DEFAULT 0`,
	kind.Decimal:  `DEFAULT 0`,
	kind.Bool:     `DEFAULT 0`,
	kind.Datetime: `DEFAULT '1970-01-01T00:00:00'`,
	kind.Date:     `DEFAULT '1970-01-01'`,
	kind.Time:     `DEFAULT '00:00:00'`,
	kind.Bytes:    `DEFAULT 0x`,
	kind.Unknown:  `DEFAULT ''`,
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
			sb.WriteString("@p")
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
