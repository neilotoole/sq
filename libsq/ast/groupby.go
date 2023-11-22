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

var _ Node = (*GroupByNode)(nil)

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
	if existing := FindNodes[*GroupByNode](v.cur.ast()); len(existing) > 0 {
		return errorf("only one group_by() clause allowed")
	}
	node := &GroupByNode{}
	node.ctx = ctx
	node.text = ctx.GetText()
	if err := v.cur.AddChild(node); err != nil {
		return err
	}

	return v.using(node, func() any {
		return v.VisitChildren(ctx)
	})
}

// VisitGroupByTerm implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitGroupByTerm(ctx *slq.GroupByTermContext) any {
	return v.VisitChildren(ctx)
}

var _ Node = (*HavingNode)(nil)

// HavingNode models the HAVING clause. It must always be preceded
// by a GROUP BY clause.
type HavingNode struct {
	baseNode
}

// VisitHaving implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitHaving(ctx *slq.HavingContext) any {
	if existing := FindNodes[*HavingNode](v.cur.ast()); len(existing) > 0 {
		return errorf("only one having() clause allowed")
	}

	// Check that the preceding node is a GroupByNode.
	if _, err := NodePrevSegmentChild[*GroupByNode](v.cur); err != nil {
		return err
	}

	node := &HavingNode{}
	node.ctx = ctx
	node.text = ctx.GetText()
	if err := v.cur.AddChild(node); err != nil {
		return err
	}

	return v.using(node, func() any {
		return v.VisitChildren(ctx)
	})
}

// AddChild implements Node.
func (n *HavingNode) AddChild(child Node) error {
	if len(n.children) > 0 {
		return errorf("having() clause can only have one child")
	}
	if err := nodesAreOnlyOfType([]Node{child}, typeExprNode); err != nil {
		return err
	}

	n.addChild(child)
	return child.SetParent(n)
}

// SetChildren implements ast.Node.
func (n *HavingNode) SetChildren(children []Node) error {
	if len(children) > 1 {
		return errorf("having() clause can only have one child")
	}
	if err := nodesAreOnlyOfType(children, typeExprNode); err != nil {
		return err
	}

	n.doSetChildren(children)
	return nil
}

// String returns a log/debug-friendly representation.
func (n *HavingNode) String() string {
	text := nodeString(n)
	return text
}
