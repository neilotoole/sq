package render

import (
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// doFromTable renders a table selector to SQL.
//
//	.actor    -->  FROM "actor"
//	.actor:a  -->  FROM "actor" "a"
func doFromTable(rc *Context, tblSel *ast.TblSelectorNode) (string, error) {
	tbl := tblSel.Table()
	if tbl.Table == "" {
		return "", errz.Errorf("selector has empty table name: {%s}", tblSel.Text())
	}

	clause := "FROM " + tbl.Render(rc.Dialect.Enquote)
	alias := tblSel.Alias()
	if alias != "" {
		clause += " AS " + rc.Dialect.Enquote(alias)
	}

	return clause, nil
}
