// Code generated from /Users/neilotoole/work/sq/sq/grammar/SLQ.g4 by ANTLR 4.7.2. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr"

type BaseSLQVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseSLQVisitor) VisitStmtList(ctx *StmtListContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitQuery(ctx *QueryContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitSegment(ctx *SegmentContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitElement(ctx *ElementContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitCmpr(ctx *CmprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFn(ctx *FnContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitJoin(ctx *JoinContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitJoinConstraint(ctx *JoinConstraintContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitGroup(ctx *GroupContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitSelElement(ctx *SelElementContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitDsTblElement(ctx *DsTblElementContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitDsElement(ctx *DsElementContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitRowRange(ctx *RowRangeContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFnName(ctx *FnNameContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitExpr(ctx *ExprContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitLiteral(ctx *LiteralContext) any {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitUnaryOperator(ctx *UnaryOperatorContext) any {
	return v.VisitChildren(ctx)
}
