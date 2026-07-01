package sqlite3

import (
	"bytes"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/stringz"
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

// buildCreateTableStmt builds a CREATE TABLE statement from tblDef. Identifiers
// are quoted via stringz.DoubleQuote (sqlite3's dialect Enquote func), which
// doubles any embedded quote char, so a name like `we"ird` cannot break out of
// its quoting.
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

func renderFuncContainsInstr(rc *render.Context, fn *ast.FuncNode) (string, error) {
	colSQL, lit, err := render.ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	return "instr(" + colSQL + ", " + stringz.SingleQuote(lit) + ") > 0", nil
}

func renderFuncStartsWithSubstr(rc *render.Context, fn *ast.FuncNode) (string, error) {
	colSQL, lit, err := render.ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	n := utf8.RuneCountInString(lit)
	return "substr(" + colSQL + ", 1, " + strconv.Itoa(n) + ") = " + stringz.SingleQuote(lit), nil
}

func renderFuncEndsWithSubstr(rc *render.Context, fn *ast.FuncNode) (string, error) {
	colSQL, lit, err := render.ParseLikeArgs(rc, fn)
	if err != nil {
		return "", err
	}
	n := utf8.RuneCountInString(lit)
	if n == 0 {
		// SQLite evaluates substr(col, -0) as substr(col, 0), which returns
		// the full string, so the naive `substr(col, -N) = ''` shape would
		// be false for every row. Emit `col LIKE '%'` to match the LIKE-based
		// drivers exactly — including NULL propagation under negation, which
		// `col IS NOT NULL` would not preserve. SQLite's default LIKE case
		// sensitivity is irrelevant here because `%` matches any character.
		return colSQL + " LIKE '%'", nil
	}
	return "substr(" + colSQL + ", -" + strconv.Itoa(n) + ") = " + stringz.SingleQuote(lit), nil
}

// SQLite's default LIKE is ASCII case-insensitive, so the i* family
// uses plain LIKE rather than the instr/substr shape that
// contains/startswith/endswith use for case-sensitivity. Non-ASCII
// characters are not case-folded — that's a SQLite limitation.

func renderFuncIContainsLike(rc *render.Context, fn *ast.FuncNode) (string, error) {
	return render.RenderLikeOp(rc, fn, render.LikeOpts{Mode: render.LikeContains})
}

func renderFuncIStartsWithLike(rc *render.Context, fn *ast.FuncNode) (string, error) {
	return render.RenderLikeOp(rc, fn, render.LikeOpts{Mode: render.LikeStartsWith})
}

func renderFuncIEndsWithLike(rc *render.Context, fn *ast.FuncNode) (string, error) {
	return render.RenderLikeOp(rc, fn, render.LikeOpts{Mode: render.LikeEndsWith})
}

// renderFuncLike renders SLQ's like and ilike on SQLite. Both
// register the same function because SQLite's default LIKE is
// ASCII case-insensitive, so the two are structurally
// indistinguishable on this driver — a documented quirk.
func renderFuncLike(rc *render.Context, fn *ast.FuncNode) (string, error) {
	return render.RenderLikeRaw(rc, fn, render.LikeRawOpts{})
}
