package ast

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// ArgNode implements the SQL "DISTINCT" clause.
type ArgNode struct {
	key string
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *ArgNode) String() string {
	return nodeString(n)
}

// Key returns the arg key. If the arg is "$name", the key is "name".
func (n *ArgNode) Key() string {
	return n.key
}

// VisitArg implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitArg(ctx *slq.ArgContext) interface{} {
	node := &ArgNode{}
	node.ctx = ctx
	node.text = ctx.GetText()

	if ctx.ARG() != nil {
		// The node text will be "$key". We need to trim the $ prefix.
		node.key = strings.TrimPrefix(ctx.ARG().GetText(), "$")
	}

	return v.cur.AddChild(node)
}
