package render

import (
	"github.com/neilotoole/sq/libsq/ast"
)

// Literal implement FragmentBuilder.
func doLiteral(rc *Context, lit *ast.LiteralNode) (string, error) {
	switch lit.LiteralType() {
	case ast.LiteralNull:
		return "NULL", nil
	case ast.LiteralNaturalNumber, ast.LiteralAnyNumber:
		return lit.Text(), nil
	case ast.LiteralString:
		text, _, err := unquoteLiteral(lit.Text())
		if err != nil {
			return "", err
		}
		return rc.Dialect.QuoteLiteral(text), nil
	case ast.LiteralBool:
		return lit.Text(), nil
	default:
		// Should never happen.
		panic("unknown literal type: " + string(lit.LiteralType()))
	}
}
