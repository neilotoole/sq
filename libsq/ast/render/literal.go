package render

import (
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// Literal implement FragmentBuilder.
func doLiteral(_ *Context, _ *Renderer, lit *ast.LiteralNode) (string, error) {
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
		return stringz.SingleQuote(text), nil
	default:
		// Should never happen.
		panic("unknown literal type: " + string(lit.LiteralType()))
	}
}
