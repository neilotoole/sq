package mysql

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/ast/sqlbuilder"
	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
)

func newFragmentBuilder(log lg.Log) *sqlbuilder.BaseFragmentBuilder {
	r := &sqlbuilder.BaseFragmentBuilder{}
	r.Log = log
	r.Quote = "`"
	r.QuoteFn = stringz.BacktickQuote
	r.Ops = sqlbuilder.BaseOps()
	return r
}

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
		panic(fmt.Sprintf("unsupported datatype %q", knd))
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
func buildCreateTableStmt(tblDef *sqlmodel.TableDef) string {
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

	uniq := ""
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

	fk := ""
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
