// Code generated from SLQ.g4 by ANTLR 4.12.0. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr/v4"

type BaseSLQVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseSLQVisitor) VisitStmtList(ctx *StmtListContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitQuery(ctx *QueryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitSegment(ctx *SegmentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitElement(ctx *ElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitCmpr(ctx *CmprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFn(ctx *FnContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFnElement(ctx *FnElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitJoin(ctx *JoinContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitJoinConstraint(ctx *JoinConstraintContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitGroup(ctx *GroupContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitSelector(ctx *SelectorContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitSelectorElement(ctx *SelectorElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitAlias(ctx *AliasContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitHandleTable(ctx *HandleTableContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitHandle(ctx *HandleContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitRowRange(ctx *RowRangeContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFnName(ctx *FnNameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitExpr(ctx *ExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitUnaryOperator(ctx *UnaryOperatorContext) interface{} {
	return v.VisitChildren(ctx)
}
