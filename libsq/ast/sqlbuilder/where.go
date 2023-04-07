package sqlbuilder

import "github.com/neilotoole/sq/libsq/ast"

func doWhere(bc *BuildContext, r *Renderer, where *ast.WhereNode) (string, error) {
	if where == nil {
		return "", nil
	}
	sql, err := r.Expr(bc, r, where.Expr())
	if err != nil {
		return "", err
	}

	sql = "WHERE " + sql
	return sql, nil
}
