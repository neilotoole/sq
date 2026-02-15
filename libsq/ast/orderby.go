package ast

import (
	"strings"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// OrderByNode implements the SQL "ORDER BY" clause.
type OrderByNode struct {
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *OrderByNode) String() string {
	return nodeString(n)
}

// Terms returns a new slice containing the ordering terms of the OrderByNode.
// The returned slice has at least one term.
func (n *OrderByNode) Terms() []*OrderByTermNode {
	terms := make([]*OrderByTermNode, len(n.children))
	for i := range n.children {
		terms[i], _ = n.children[i].(*OrderByTermNode)
	}
	return terms
}

// AddChild implements Node.AddChild. It returns an error
// if child is nil or not type OrderByTermNode.
func (n *OrderByNode) AddChild(child Node) error {
	term, ok := child.(*OrderByTermNode)
	if !ok {
		return errorf("can only add child of type %T to %T but got %T", term, n, child)
	}

	n.addChild(child)
	return nil
}

// SetChildren implements ast.Node.
func (n *OrderByNode) SetChildren(children []Node) error {
	if len(children) > 1 {
		return errorf("%T can have only 1 child but attempt to set %d children",
			n, len(children))
	}

	for i := range children {
		if _, ok := children[i].(*OrderByTermNode); !ok {
			return errorf("illegal child type %T {%s} for %T", children[i], children[i], n)
		}
	}

	n.doSetChildren(children)
	return nil
}

// OrderByDirection specifies the "ORDER BY" direction.
type OrderByDirection string

const (
	// OrderByDirectionNone is the default order direction.
	OrderByDirectionNone OrderByDirection = ""

	// OrderByDirectionAsc is the ascending  (DESC) order direction.
	OrderByDirectionAsc OrderByDirection = "ASC"

	// OrderByDirectionDesc is the descending (DESC) order direction.
	OrderByDirectionDesc OrderByDirection = "DESC"
)

// OrderByTermNode is a child of OrderByNode.
type OrderByTermNode struct {
	direction OrderByDirection
	baseNode
}

// AddChild accepts a single child of type *SelectorNode.
func (n *OrderByTermNode) AddChild(child Node) error {
	if len(n.children) > 0 {
		return errorf("%T is only allowed a single child", n)
	}

	selNode, ok := child.(*SelectorNode)
	if !ok {
		return errorf("illegal %T child type %T: %s", n, child, child)
	}

	n.addChild(selNode)
	return child.SetParent(n)
}

// SetChildren implements ast.Node.
func (n *OrderByTermNode) SetChildren(children []Node) error {
	switch len(children) {
	case 0:
		// fallthrough
	case 1:
		if _, ok := children[0].(Selector); !ok {
			return errorf("illegal child type %T {%s} for %T", children[0], children[0], n)
		}
	default:
		return errorf("%T can have only 1 child but attempt to set %d children",
			n, len(children))
	}

	n.doSetChildren(children)
	return nil
}

// Selector returns the ordering term's selector.
func (n *OrderByTermNode) Selector() Node {
	return n.children[0]
}

// Direction returns the ordering term's direction.
func (n *OrderByTermNode) Direction() OrderByDirection {
	return n.direction
}

// String returns a log/debug-friendly representation.
func (n *OrderByTermNode) String() string {
	return nodeString(n)
}

// VisitOrderBy implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitOrderBy(ctx *slq.OrderByContext) any {
	if existing := FindNodes[*OrderByNode](v.cur.ast()); len(existing) > 0 {
		return errorf("only one order_by() clause allowed")
	}

	node := &OrderByNode{}
	node.parent = v.cur
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

// VisitOrderByTerm implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitOrderByTerm(ctx *slq.OrderByTermContext) any {
	node := &OrderByTermNode{}
	node.parent = v.cur
	node.ctx = ctx
	node.text = ctx.GetText()
	if before, ok := strings.CutSuffix(node.text, "+"); ok {
		node.text = before
		node.direction = OrderByDirectionAsc
	} else if before, ok := strings.CutSuffix(node.text, "-"); ok {
		node.text = before
		node.direction = OrderByDirectionDesc
	}

	selNode, err := newSelectorNode(node, ctx.Selector())
	if err != nil {
		return nil
	}

	if err = node.AddChild(selNode); err != nil {
		return err
	}

	return v.cur.AddChild(node)
}
