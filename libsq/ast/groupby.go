package ast

import (
	"reflect"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

var groupByAllowedChildren = []reflect.Type{
	typeSelectorNode,
	typeColSelectorNode,
	typeTblColSelectorNode,
	typeFuncNode,
}

// GroupByNode models GROUP BY. The children of GroupBy node can be
// of type selector or FuncNode.
type GroupByNode struct {
	baseNode
}

// AddChild implements Node.
func (n *GroupByNode) AddChild(child Node) error {
	if err := nodesAreOnlyOfType([]Node{child}, groupByAllowedChildren...); err != nil {
		return err
	}

	n.addChild(child)
	return child.SetParent(n)
}

// SetChildren implements ast.Node.
func (n *GroupByNode) SetChildren(children []Node) error {
	if err := nodesAreOnlyOfType(children, groupByAllowedChildren...); err != nil {
		return err
	}

	n.doSetChildren(children)
	return nil
}

// String returns a log/debug-friendly representation.
func (n *GroupByNode) String() string {
	text := nodeString(n)
	return text
}

// VisitGroupBy implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitGroupBy(ctx *slq.GroupByContext) any {
	node := &GroupByNode{}
	node.ctx = ctx
	node.text = ctx.GetText()
	if err := v.cur.AddChild(node); err != nil {
		return err
	}

	return v.using(node, func() any {
		// This will result in VisitOrderByTerm being called on the children.
		return v.VisitChildren(ctx)
	})
}

// VisitGroupByTerm implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitGroupByTerm(ctx *slq.GroupByTermContext) interface{} {
	return v.VisitChildren(ctx)
}
