package render

import "github.com/neilotoole/sq/libsq/ast"

func doWhere(rc *Context, where *ast.WhereNode) (string, error) {
	if where == nil {
		return "", nil
	}
	sql, err := rc.Renderer.Expr(rc, where.Expr())
	if err != nil {
		return "", err
	}

	sql = "WHERE " + sql
	return sql, nil
}
