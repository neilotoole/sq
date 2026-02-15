package ast

import "github.com/neilotoole/sq/libsq/ast/internal/slq"

// WhereNode represents a SQL WHERE clause, i.e. a filter on the SELECT.
type WhereNode struct {
	baseNode
}

// String returns a log/debug-friendly representation.
func (n *WhereNode) String() string {
	return nodeString(n)
}

// Expr returns the expression that constitutes the SetWhere clause, or nil if no expression.
func (n *WhereNode) Expr() *ExprNode {
	if len(n.children) == 0 {
		return nil
	}

	return n.children[0].(*ExprNode)
}

// AddChild implements Node.
func (n *WhereNode) AddChild(node Node) error {
	expr, ok := node.(*ExprNode)
	if !ok {
		return errorf("WHERE child must be %T, but got: %T", expr, node)
	}

	if len(n.children) > 0 {
		return errorf("WHERE has max 1 child: failed to add: %T", node)
	}

	n.addChild(expr)
	return nil
}

// VisitWhere implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitWhere(ctx *slq.WhereContext) any {
	node := &WhereNode{}
	node.ctx = ctx
	node.text = ctx.GetText()

	if e := v.using(node, func() any {
		return v.VisitChildren(ctx)
	}); e != nil {
		return e
	}

	if len(node.Children()) == 0 {
		return errorf("{%s} requires at least one argument", ctx.WHERE().GetText())
	}

	return v.cur.AddChild(node)
}
