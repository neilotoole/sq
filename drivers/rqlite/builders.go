// DDL/DML builder helpers for the rqlite driver.
//
// Currently identical to drivers/sqlite3's same-named helpers; drift risk
// if either changes. Extract to a shared package if a third driver needs
// them. They are duplicated rather than imported because drivers in this
// repo do not cross-import.

package rqlite

import (
	"bytes"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// createTblKindDefaults maps Kind to the DEFAULT clause emitted in a
// CREATE TABLE statement. nolint:exhaustive ignores kind.Null on purpose.
var createTblKindDefaults = map[kind.Kind]string{ //nolint:exhaustive
	kind.Text:     `DEFAULT ''`,
	kind.Int:      `DEFAULT 0`,
	kind.Float:    `DEFAULT 0`,
	kind.Decimal:  `DEFAULT 0`,
	kind.Bool:     `DEFAULT 0`,
	kind.Datetime: "DEFAULT '1970-01-01T00:00:00'",
	kind.Date:     "DEFAULT '1970-01-01'",
	kind.Time:     "DEFAULT '00:00'",
	kind.Bytes:    "DEFAULT ''",
	kind.Unknown:  `DEFAULT ''`,
}

// buildCreateTableStmt emits a CREATE TABLE statement for tblDef using
// SQLite syntax (which rqlite executes verbatim).
func buildCreateTableStmt(tblDef *schema.Table) string {
	var buf *bytes.Buffer

	cols := make([]string, len(tblDef.Cols))
	for i, col := range tblDef.Cols {
		buf = &bytes.Buffer{}
		buf.WriteString(stringz.DoubleQuote(col.Name))
		buf.WriteRune(' ')
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

	var fk string
	buf = &bytes.Buffer{}
	for _, col := range tblDef.Cols {
		if col.ForeignKey == nil {
			continue
		}

		if buf.Len() > 0 {
			buf.WriteString(",\n")
		}

		fkName := tblDef.Name + "_" + col.Name + "_" +
			col.ForeignKey.RefTable + "_" + col.ForeignKey.RefCol + "_fk"
		buf.WriteString(`CONSTRAINT `)
		buf.WriteString(stringz.DoubleQuote(fkName))
		buf.WriteString(` FOREIGN KEY (`)
		buf.WriteString(stringz.DoubleQuote(col.Name))
		buf.WriteString(`) REFERENCES `)
		buf.WriteString(stringz.DoubleQuote(col.ForeignKey.RefTable))
		buf.WriteString(` (`)
		buf.WriteString(stringz.DoubleQuote(col.ForeignKey.RefCol))
		buf.WriteString(`) ON DELETE `)
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
	buf.WriteString(`CREATE TABLE `)
	buf.WriteString(stringz.DoubleQuote(tblDef.Name))
	buf.WriteString(" (\n")

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
	return buf.String()
}

// buildUpdateStmt emits an UPDATE statement with placeholders for each
// listed column. `where` is appended verbatim (must already be SQL).
func buildUpdateStmt(tbl string, cols []string, where string) (string, error) {
	if len(cols) == 0 {
		return "", errz.Errorf("no columns provided")
	}

	buf := strings.Builder{}
	buf.WriteString(`UPDATE `)
	buf.WriteString(stringz.DoubleQuote(tbl))
	buf.WriteString(` SET `)
	for i, col := range cols {
		if i > 0 {
			buf.WriteString(`, `)
		}
		buf.WriteString(stringz.DoubleQuote(col))
		buf.WriteString(` = ?`)
	}
	if where != "" {
		buf.WriteString(" WHERE ")
		buf.WriteString(where)
	}

	return buf.String(), nil
}
