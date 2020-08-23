package sqlite3

import (
	"bytes"

	"github.com/neilotoole/sq/libsq/sqlbuilder"

	"strings"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/sqlmodel"
)

func newFragmentBuilder(log lg.Log) *sqlbuilder.BaseFragmentBuilder {
	return &sqlbuilder.BaseFragmentBuilder{Log: log, Quote: `"`, ColQuote: `"`, Ops: sqlbuilder.BaseOps()}
}

// createTblKindDefaults is a mapping of Kind to the value
// to use for a column's DEFAULT clause in a CREATE TABLE statement.
var createTblKindDefaults = map[sqlz.Kind]string{
	sqlz.KindText:     `DEFAULT ''`,
	sqlz.KindInt:      `DEFAULT 0`,
	sqlz.KindFloat:    `DEFAULT 0`,
	sqlz.KindDecimal:  `DEFAULT 0`,
	sqlz.KindBool:     `DEFAULT 0`,
	sqlz.KindDatetime: "DEFAULT '1970-01-01T00:00:00'",
	sqlz.KindDate:     "DEFAULT '1970-01-01'",
	sqlz.KindTime:     "DEFAULT '00:00'",
	sqlz.KindBytes:    "DEFAULT ''",
	sqlz.KindUnknown:  `DEFAULT ''`,
}

func buildCreateTableStmt(tblDef *sqlmodel.TableDef) (string, error) {
	var buf *bytes.Buffer

	cols := make([]string, len(tblDef.Cols))
	for i, col := range tblDef.Cols {
		buf = &bytes.Buffer{}
		buf.WriteRune('"')
		buf.WriteString(col.Name)
		buf.WriteString(`" `)
		buf.WriteString(DBTypeForKind(col.Kind))

		if col.Name == tblDef.PKColName {
			buf.WriteString(" PRIMARY KEY")
			if tblDef.AutoIncrement {
				buf.WriteString(" AUTOINCREMENT")
			}
		}

		if col.HasDefault {
			buf.WriteRune(' ')
			buf.WriteString(createTblKindDefaults[col.Kind])
		}

		if col.NotNull {
			buf.WriteString(" NOT NULL")
		}

		if col.Unique {
			buf.WriteString(" UNIQUE")
		}

		cols[i] = buf.String()
	}

	fk := ""
	buf = &bytes.Buffer{}
	for _, col := range tblDef.Cols {
		if col.ForeignKey == nil {
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString(",\n")
		}

		buf.WriteString(`CONSTRAINT "`)
		buf.WriteString(tblDef.Name)
		buf.WriteRune('_')
		buf.WriteString(col.Name)
		buf.WriteRune('_')
		buf.WriteString(col.ForeignKey.RefTable)
		buf.WriteRune('_')
		buf.WriteString(col.ForeignKey.RefCol)
		buf.WriteString(`_fk" FOREIGN KEY ("`)
		buf.WriteString(col.Name)
		buf.WriteString(`") REFERENCES "`)
		buf.WriteString(col.ForeignKey.RefTable)
		buf.WriteString(`" ("`)
		buf.WriteString(col.ForeignKey.RefCol)
		buf.WriteString(`") ON DELETE `)
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

	buf = &bytes.Buffer{}
	buf.WriteString(`CREATE TABLE "`)
	buf.WriteString(tblDef.Name)
	buf.WriteString("\" (\n")

	for x := 0; x < len(cols)-1; x++ {
		buf.WriteString(cols[x])
		buf.WriteString(",\n")
	}
	buf.WriteString(cols[len(cols)-1])

	if fk != "" {
		buf.WriteString(",\n")
		buf.WriteString(fk)
	}
	buf.WriteString("\n)")
	return buf.String(), nil
}

func buildUpdateStmt(tbl string, cols []string, where string) (string, error) {
	if len(cols) == 0 {
		return "", errz.Errorf("no columns provided")
	}

	buf := strings.Builder{}
	buf.WriteString(`UPDATE "`)
	buf.WriteString(tbl)
	buf.WriteString(`" SET "`)
	buf.WriteString(strings.Join(cols, `" = ?, "`))
	buf.WriteString(`" = ?`)
	if where != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(where)
	}

	return buf.String(), nil
}
