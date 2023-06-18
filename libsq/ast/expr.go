package ast

import "github.com/neilotoole/sq/libsq/ast/internal/slq"

var (
	_ Node         = (*ExprElementNode)(nil)
	_ ResultColumn = (*ExprElementNode)(nil)
)

// ExprElementNode is an expression that acts as a ResultColumn.
//
//	.actor | (1+2):alias
//
// In the example above, the expression "(1+2)" is rendered with
// an alias, e.g.
//
//	SELECT 1+2 AS "alias" FROM "actor"
type ExprElementNode struct {
	baseNode
	alias    string
	exprNode *ExprNode
}

// String returns a log/debug-friendly representation.
func (ex *ExprElementNode) String() string {
	str := nodeString(ex)
	if ex.alias != "" {
		str += ":" + ex.alias
	}
	return str
}

// Text implements ResultColumn.
func (ex *ExprElementNode) Text() string {
	return ex.ctx.GetText()
}

// Alias implements ResultColumn.
func (ex *ExprElementNode) Alias() string {
	return ex.alias
}

// ExprNode returns the child expression.
func (ex *ExprElementNode) ExprNode() *ExprNode {
	return ex.exprNode
}

// SetChildren implements Node.
func (ex *ExprElementNode) SetChildren(children []Node) error {
	ex.setChildren(children)
	return nil
}

// AddChild implements Node.
func (ex *ExprElementNode) AddChild(child Node) error {
	// TODO: add check for valid ExprElementNode child types
	ex.addChild(child)
	return child.SetParent(ex)
}

// VisitExprElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitExprElement(ctx *slq.ExprElementContext) interface{} {
	childCount := ctx.GetChildCount()
	if childCount == 0 || childCount > 2 {
		return errorf("parser: invalid expression: expected 1 or 2 children, but got %d: %v",
			childCount, ctx.GetText())
	}

	node := &ExprElementNode{}
	node.ctx = ctx
	node.text = ctx.GetText()
	if err := node.SetParent(v.cur); err != nil {
		return err
	}

	if ctx.Expr() == nil {
		return errorf("parser: invalid expression: %s", ctx.GetText())
	}

	if e := v.using(node, func() any {
		exprCtx, ok := ctx.Expr().(*slq.ExprContext)
		if !ok || exprCtx == nil {
			return errorf("parser: invalid expression: %s", ctx.GetText())
		}
		return v.VisitExpr(exprCtx)
	}); e != nil {
		return e
	}

	var ok bool
	if node.exprNode, ok = node.children[0].(*ExprNode); !ok {
		return errorf("parser: invalid expression: %s", ctx.GetText())
	}

	if ctx.Alias() != nil {
		aliasCtx, ok := ctx.Alias().(*slq.AliasContext)
		if !ok {
			return errorf("expected second child to be %T but was %T: %v", aliasCtx, ctx.Alias(), ctx.GetText())
		}
		if e := v.using(node, func() any {
			return v.VisitAlias(aliasCtx)
		}); e != nil {
			return e
		}
	}
	return v.cur.AddChild(node)
}

// ExprNode models a SLQ expression such as ".uid > 4".
type ExprNode struct {
	baseNode
	parens bool
}

// HasParens returns true if the expression is enclosed in parentheses.
func (n *ExprNode) HasParens() bool {
	return n.parens
}

// AddChild implements Node.
func (n *ExprNode) AddChild(child Node) error {
	n.addChild(child)
	return child.SetParent(n)
}

// SetChildren implements Node.
func (n *ExprNode) SetChildren(children []Node) error {
	n.setChildren(children)
	return nil
}

// String returns a log/debug-friendly representation.
func (n *ExprNode) String() string {
	text := nodeString(n)
	return text
}

// VisitExpr implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitExpr(ctx *slq.ExprContext) any {
	// check if the expr is a selector, e.g. ".uid"
	if selCtx := ctx.Selector(); selCtx != nil {
		selNode, err := newSelectorNode(v.cur, selCtx)
		if err != nil {
			return err
		}
		return v.cur.AddChild(selNode)
	}

	node := &ExprNode{}
	node.ctx = ctx
	node.text = ctx.GetText()
	if err := node.SetParent(v.cur); err != nil {
		return err
	}
	var err error
	if node.parens, err = exprHasParens(ctx); err != nil {
		return err
	}

	if e := v.using(node, func() any {
		return v.VisitChildren(ctx)
	}); e != nil {
		return e
	}

	return v.cur.AddChild(node)
}

func exprHasParens(ctx *slq.ExprContext) (bool, error) {
	if ctx == nil {
		return false, errorf("expression context is nil")
	}

	lpar := ctx.LPAR()
	rpar := ctx.RPAR()

	switch {
	case lpar == nil && rpar == nil:
		return false, nil
	case lpar != nil && rpar != nil:
		return true, nil
	default:
		return false, errorf("unbalanced parenthesis: %s", ctx.GetText())
	}
}
