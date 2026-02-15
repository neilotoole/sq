package ast

import "github.com/neilotoole/sq/libsq/ast/internal/slq"

// UniqueNode implements the SQL "DISTINCT" clause.
type UniqueNode struct {
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *UniqueNode) String() string {
	return nodeString(n)
}

// VisitUniqueFunc implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitUniqueFunc(ctx *slq.UniqueFuncContext) any {
	node := &UniqueNode{}
	node.ctx = ctx
	node.text = ctx.GetText()
	return v.cur.AddChild(node)
}
