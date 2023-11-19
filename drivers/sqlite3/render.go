package sqlite3

import (
	"bytes"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"

	"github.com/neilotoole/sq/libsq/core/kind"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
)

// createTblKindDefaults is a mapping of Kind to the value
// to use for a column's DEFAULT clause in a CREATE TABLE statement.
var createTblKindDefaults = map[kind.Kind]string{ //nolint:exhaustive // ignore kind.Null
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

func buildCreateTableStmt(tblDef *sqlmodel.TableDef) string {
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
	return buf.String()
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

func doRenderFuncSchema(_ *render.Context, fn *ast.FuncNode) (string, error) {
	if fn.FuncName() != ast.FuncNameSchema {
		// Shouldn't happen
		return "", errz.Errorf("expected %s function, got %q", ast.FuncNameSchema, fn.FuncName())
	}

	const frag = `(SELECT name FROM pragma_database_list ORDER BY seq limit 1)`
	return frag, nil
}

// doRenderFuncCatalog renders the catalog function. SQLite doesn't
// support catalogs, so we just return the string "default". We could
// return empty string, but that may be even more confusing, and would
// make SQLite the odd man out, as the other SQL drivers (even MySQL)
// have a value for catalog.
func doRenderFuncCatalog(_ *render.Context, fn *ast.FuncNode) (string, error) {
	if fn.FuncName() != ast.FuncNameCatalog {
		// Shouldn't happen
		return "", errz.Errorf("expected %s function, got %q", ast.FuncNameCatalog, fn.FuncName())
	}

	const frag = `(SELECT 'default')`
	return frag, nil
}
