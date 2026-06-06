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
	"github.com/neilotoole/sq/libsq/source/metadata"
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

	var fk string
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
	return buf.String()
}

// buildUpdateStmt emits an UPDATE statement with placeholders for each
// listed column. `where` is appended verbatim (must already be SQL).
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

// tableMetadataToSchema converts a *metadata.Table (the shape returned
// by getTableMetadata) into a *schema.Table (the shape consumed by
// buildCreateTableStmt). It preserves only what sq's *schema.Column
// models from the metadata side: Name, Kind, NotNull (from !Nullable),
// and HasDefault (as a boolean; the original default expression is
// NOT preserved). PKColName is set from the FIRST column with
// PrimaryKey=true.
//
// The following are NOT preserved because metadata.Column does not
// expose them or schema.Column does not model them:
//   - UNIQUE column constraints
//   - FOREIGN KEY constraints
//   - AUTOINCREMENT on the primary key (metadata.Column has no signal
//     for it)
//   - The original DEFAULT expression value (substituted by a canned
//     per-kind default from createTblKindDefaults)
//   - CHECK constraints, indexes, triggers
//
// Callers using this for CopyTable / AlterTableColumnKinds: be aware
// that the rebuilt table will have these lossy substitutions applied.
func tableMetadataToSchema(md *metadata.Table, newName string) *schema.Table {
	tblDef := &schema.Table{
		Name: newName,
		Cols: make([]*schema.Column, len(md.Columns)),
	}
	for i, mcol := range md.Columns {
		col := &schema.Column{
			Table:      tblDef,
			Name:       mcol.Name,
			Kind:       mcol.Kind,
			NotNull:    !mcol.Nullable,
			HasDefault: mcol.DefaultValue != "",
		}
		tblDef.Cols[i] = col
		if mcol.PrimaryKey && tblDef.PKColName == "" {
			tblDef.PKColName = mcol.Name
		}
	}
	return tblDef
}
