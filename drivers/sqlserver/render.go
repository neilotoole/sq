package sqlserver

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
)

func renderRange(_ *render.Context, rr *ast.RowRangeNode) (string, error) {
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

	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("OFFSET %d ROWS", offset))

	if rr.Limit > -1 {
		buf.WriteString(fmt.Sprintf(" FETCH NEXT %d ROWS ONLY", rr.Limit))
	}

	sql := buf.String()
	return sql, nil
}

func preRender(_ *render.Context, f *render.Fragments) error {
	// SQL Server handles range (OFFSET, LIMIT) a little differently. If the query has a range,
	// then the ORDER BY clause is required. If ORDER BY is not specified, we use a trick (SELECT 0)
	// to satisfy SQL Server. For example:
	//
	//   SELECT * FROM actor
	//   ORDER BY (SELECT 0)
	//   OFFSET 1 ROWS
	//   FETCH NEXT 2 ROWS ONLY;
	if f.Range != "" {
		if f.OrderBy == "" {
			f.OrderBy = "ORDER BY (SELECT 0)"
		}
	}

	return nil
}

func dbTypeNameFromKind(knd kind.Kind) string {
	switch knd { //nolint:exhaustive // ignore kind.Null
	default:
		panic(fmt.Sprintf("unsupported datatype {%s}", knd))
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
func buildCreateTableStmt(tblDef *schema.Table) string {
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
		}

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

	return sb.String()
}

// renderFuncRowNum renders the rownum() function.
func renderFuncRowNum(rc *render.Context, fn *ast.FuncNode) (string, error) {
	a, _ := ast.NodeRoot(fn).(*ast.AST)
	obNode := ast.FindFirstNode[*ast.OrderByNode](a)
	if obNode != nil {
		obClause, err := rc.Renderer.OrderBy(rc, obNode)
		if err != nil {
			return "", err
		}
		return "(row_number() OVER (" + obClause + "))", nil
	}

	// The following is a hack to get around the fact that SQL Server
	// requires an ORDER BY clause window functions.
	// See:
	// - https://stackoverflow.com/a/33013690/6004734
	// - https://stackoverflow.com/a/50645278/6004734
	return "(row_number() OVER (ORDER BY (SELECT NULL)))", nil
}
