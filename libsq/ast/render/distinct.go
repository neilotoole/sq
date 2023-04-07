package render

import "github.com/neilotoole/sq/libsq/ast"

func doDistinct(_ *Context, n *ast.UniqueNode) (string, error) {
	if n == nil {
		return "", nil
	}
	return "DISTINCT", nil
}
