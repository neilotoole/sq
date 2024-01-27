package ast

import (
	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// LiteralType is an enum of literal types.
type LiteralType string

const (
	LiteralNull          LiteralType = "null"
	LiteralNaturalNumber LiteralType = "int"
	LiteralAnyNumber     LiteralType = "float"
	LiteralString        LiteralType = "string"
)

// LiteralNode is a leaf node representing a literal such as a number or a string.
type LiteralNode struct {
	typ LiteralType
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *LiteralNode) String() string {
	return nodeString(n)
}

// LiteralType returns the literal type (number, string, etc).
func (n *LiteralNode) LiteralType() LiteralType {
	return n.typ
}

// VisitLiteral implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitLiteral(ctx *slq.LiteralContext) any {
	node := &LiteralNode{}
	node.ctx = ctx
	node.text = ctx.GetText()

	switch {
	case ctx.NN() != nil:
		node.typ = LiteralNaturalNumber
	case ctx.NUMBER() != nil:
		node.typ = LiteralAnyNumber
	case ctx.NULL() != nil:
		node.typ = LiteralNull
	case ctx.STRING() != nil:
		node.typ = LiteralString
	default:
		// Shouldn't happen
		return errorf("unable to determine literal type for: %s", ctx.GetText())
	}

	if err := node.SetParent(v.cur); err != nil {
		return err
	}
	return v.cur.AddChild(node)
}
