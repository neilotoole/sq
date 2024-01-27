package mysql

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

func dbTypeNameFromKind(knd kind.Kind) string {
	switch knd { //nolint:exhaustive // ignore kind.Unknown and kind.Null
	case kind.Text:
		return "TEXT"
	case kind.Int:
		return "INT"
	case kind.Float:
		return "DOUBLE"
	case kind.Decimal:
		return "DECIMAL"
	case kind.Bool:
		return "TINYINT(1)"
	case kind.Datetime:
		return "DATETIME"
	case kind.Time:
		return "TIME"
	case kind.Date:
		return "DATE"
	case kind.Bytes:
		return "BLOB"
	default:
		panic(fmt.Sprintf("unsupported data kind {%s}", knd))
	}
}

// createTblKindDefaults is a map of Kind to the value
// to use for a column's DEFAULT clause in a CREATE TABLE statement.
//
// Note that MySQL (at least of v5.6) doesn't support DEFAULT values
// for TEXT or BLOB columns.
// https://bugs.mysql.com/bug.php?id=21532
var createTblKindDefaults = map[kind.Kind]string{ //nolint:exhaustive
	kind.Text:     ``,
	kind.Int:      `DEFAULT 0`,
	kind.Float:    `DEFAULT 0`,
	kind.Decimal:  `DEFAULT 0`,
	kind.Bool:     `DEFAULT 0`,
	kind.Datetime: `DEFAULT '1970-01-01 00:00:00'`,
	kind.Date:     `DEFAULT '1970-01-01'`,
	kind.Time:     `DEFAULT '00:00:00'`,
	kind.Bytes:    ``,
	kind.Unknown:  ``,
}

//nolint:funlen
func buildCreateTableStmt(tblDef *schema.Table) string {
	buf := &bytes.Buffer{}

	cols := make([]string, len(tblDef.Cols))
	for i, col := range tblDef.Cols {
		buf.WriteRune('`')
		buf.WriteString(col.Name)
		buf.WriteString("` ")
		buf.WriteString(dbTypeNameFromKind(col.Kind))

		if col.HasDefault {
			buf.WriteRune(' ')
			buf.WriteString(createTblKindDefaults[col.Kind])
		}

		if col.NotNull {
			buf.WriteString(" NOT NULL")
		}

		if col.Name == tblDef.PKColName && tblDef.AutoIncrement {
			buf.WriteString(" AUTO_INCREMENT")
		}
		cols[i] = buf.String()
		buf.Reset()
	}

	pk := ""
	if tblDef.PKColName != "" {
		buf.WriteString("PRIMARY KEY (`")
		buf.WriteString(tblDef.PKColName)
		buf.WriteString("`),\n")
		buf.WriteString("UNIQUE KEY `")
		buf.WriteString(tblDef.Name)
		buf.WriteRune('_')
		buf.WriteString(tblDef.PKColName)
		buf.WriteString("_uindex` (`")
		buf.WriteString(tblDef.PKColName)
		buf.WriteString("`)")
		pk = buf.String()
	}

	var uniq string
	buf.Reset()
	for _, col := range tblDef.Cols {
		if col.Name == tblDef.PKColName {
			// if the table has a PK, then we've already added a unique constraint for it above
			continue
		}

		if col.Unique {
			if buf.Len() > 0 {
				buf.WriteString(",\n")
			}
			buf.WriteString("UNIQUE KEY `")
			buf.WriteString(tblDef.Name)
			buf.WriteRune('_')
			buf.WriteString(col.Name)
			buf.WriteString("_uindex` (`")
			buf.WriteString(col.Name)
			buf.WriteString("`)")
		}
	}
	uniq = buf.String()

	var fk string
	buf.Reset()
	for _, col := range tblDef.Cols {
		if col.ForeignKey == nil {
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString("KEY `")
		buf.WriteString(tblDef.Name)
		buf.WriteRune('_')
		buf.WriteString(col.Name)
		buf.WriteRune('_')
		buf.WriteString(col.ForeignKey.RefTable)
		buf.WriteRune('_')
		buf.WriteString(col.ForeignKey.RefCol)
		buf.WriteString("_key` (`")
		buf.WriteString(col.Name)
		buf.WriteString("`),\nCONSTRAINT `")
		buf.WriteString(tblDef.Name)
		buf.WriteRune('_')
		buf.WriteString(col.Name)
		buf.WriteRune('_')
		buf.WriteString(col.ForeignKey.RefTable)
		buf.WriteRune('_')
		buf.WriteString(col.ForeignKey.RefCol)
		buf.WriteString("_fk` FOREIGN KEY (`")
		buf.WriteString(col.Name)
		buf.WriteString("`) REFERENCES `")
		buf.WriteString(col.ForeignKey.RefTable)
		buf.WriteString("` (`")
		buf.WriteString(col.ForeignKey.RefCol)
		buf.WriteString("`) ON DELETE ")
		if col.ForeignKey.OnDelete == "" {
			buf.WriteString("CASCADE")
		} else {
			buf.WriteString(col.ForeignKey.OnDelete)
		}
		buf.WriteString(" ON UPDATE ")
		if col.ForeignKey.OnUpdate == "" {
			buf.WriteString("CASCADE")
		} else {
			buf.WriteString(col.ForeignKey.OnUpdate)
		}
	}
	fk = buf.String()

	buf.Reset()
	buf.WriteString("CREATE TABLE `")
	buf.WriteString(tblDef.Name)
	buf.WriteString("` (\n")

	for x := 0; x < len(cols)-1; x++ {
		buf.WriteString(cols[x])
		buf.WriteString(",\n")
	}
	buf.WriteString(cols[len(cols)-1])

	if pk != "" {
		buf.WriteString(",\n")
		buf.WriteString(pk)
	}
	if uniq != "" {
		buf.WriteString(",\n")
		buf.WriteString(uniq)
	}
	if fk != "" {
		buf.WriteString(",\n")
		buf.WriteString(fk)
	}
	buf.WriteString("\n)")
	return buf.String()
}

func buildUpdateStmt(tbl string, cols []string, where string) (string, error) {
	if len(cols) == 0 {
		return "", errz.Errorf("no columns provided")
	}

	buf := strings.Builder{}
	buf.WriteString("UPDATE `")
	buf.WriteString(tbl)
	buf.WriteString("` SET `")
	buf.WriteString(strings.Join(cols, "` = ?, `"))
	buf.WriteString("` = ?")
	if where != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(where)
	}

	return buf.String(), nil
}

// renderFuncRowNum renders the rownum() function.
//
// MySQL didn't introduce ROW_NUMBER() until 8.0, and we're still
// trying to support 5.6 and 5.7. So, we're using a hack, as
// described here: https://www.mysqltutorial.org/mysql-row_number/
//
//	SET @row_number = 0;
//	SELECT
//	(@row_number:=@row_number + 1) AS num,
//		actor_id,
//		first_name,
//		last_name
//	FROM actor
//	ORDER BY first_name
//
// The function puts the SET statement into the PreExecStmts fragment,
// which then gets executed before the main body of the query.
//
// For MySQL 8+, we could use the ROW_NUMBER() function, but right now
// the code isn't really set up to execute different impls for different
// driver versions. Although, this is probably something we need to face up to.
func renderFuncRowNum(rc *render.Context, _ *ast.FuncNode) (string, error) { //nolint:unparam
	// We use a unique variable name to avoid collisions if there are
	// multiple uses of rownum() in the same query.
	variable := "@row_number_" + stringz.Uniq8()

	rc.Fragments.PreExecStmts = append(rc.Fragments.PreExecStmts, "SET "+variable+" = 0;")
	rc.Fragments.PostExecStmts = append(rc.Fragments.PostExecStmts, "SET "+variable+" = NULL;")

	// e.g. (@row_number_abcd1234:=@row_number_abcd1234 + 1)
	return "(" + variable + ":=" + variable + " + 1)", nil
}
