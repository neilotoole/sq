package render

import (
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func doFromTable(bc *BuildContext, _ *Renderer, tblSel *ast.TblSelectorNode) (string, error) {
	tblName, _ := tblSel.SelValue()
	if tblName == "" {
		return "", errz.Errorf("selector has empty table name: {%s}", tblSel.Text())
	}

	clause := "FROM " + bc.Dialect.Enquote(tblSel.TblName())
	return clause, nil
}
