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
	tblName := tblSel.TblName()
	if tblName == "" {
		return "", errz.Errorf("selector has empty table name: {%s}", tblSel.Text())
	}

	clause := "FROM " + rc.Dialect.Enquote(tblName)
	alias := tblSel.Alias()
	if alias != "" {
		clause += " " + rc.Dialect.Enquote(alias)
	}

	return clause, nil
}
