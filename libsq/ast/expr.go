package ast

import "github.com/neilotoole/sq/libsq/ast/internal/slq"

// VisitExprElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitExprElement(ctx *slq.ExprElementContext) interface{} {
	childCount := ctx.GetChildCount()
	if childCount == 0 || childCount > 2 {
		return errorf("parser: invalid expression: expected 1 or 2 children, but got %d: %v",
			childCount, ctx.GetText())
	}

	exprCtx, ok := ctx.Expr().(*slq.ExprContext)
	if !ok || exprCtx == nil {
		return errorf("parser: invalid expression: %s", ctx.GetText())
	}

	if e := v.VisitExpr(exprCtx); e != nil {
		return e
	}

	//// e.g. count(*)
	//child1 := ctx.GetChild(0)
	//fnCtx, ok := child1.(*slq.ExprContext)
	//if !ok {
	//	return errorf("expected first child to be %T but was %T: %v", fnCtx, child1, ctx.GetText())
	//}
	//
	//if err := v.VisitFunc(fnCtx); err != nil {
	//	return err
	//}
	//
	//// Check if there's an alias
	//if childCount == 2 {
	//	child2 := ctx.GetChild(1)
	//	aliasCtx, ok := child2.(*slq.AliasContext)
	//	if !ok {
	//		return errorf("expected second child to be %T but was %T: %v", aliasCtx, child2, ctx.GetText())
	//	}
	//
	//	// VisitAlias will expect v.cur to be a FuncNode.
	//	lastNode := nodeLastChild(v.cur)
	//	fnNode, ok := lastNode.(*FuncNode)
	//	if !ok {
	//		return errorf("expected %T but got %T: %v", fnNode, lastNode, ctx.GetText())
	//	}
	//
	//	return v.using(fnNode, func() any {
	//		return v.VisitAlias(aliasCtx)
	//	})
	//}

	return nil
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

	if ctx.Literal() != nil {
		return v.VisitLiteral(ctx.Literal().(*slq.LiteralContext))
	}

	ex := &ExprNode{}
	ex.ctx = ctx
	err := ex.SetParent(v.cur)
	if err != nil {
		return err
	}

	prev := v.cur
	v.cur = ex

	err2 := v.VisitChildren(ctx)
	v.cur = prev
	if err2 != nil {
		return err2.(error)
	}

	return v.cur.AddChild(ex)
}
