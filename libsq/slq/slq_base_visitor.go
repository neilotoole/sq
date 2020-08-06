// Code generated from /Users/neilotoole/work/moi/go/src/github.com/neilotoole/sq/grammar/SLQ.g4 by ANTLR 4.7.2. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr"

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

func (v *BaseSLQVisitor) VisitJoin(ctx *JoinContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitJoinConstraint(ctx *JoinConstraintContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitGroup(ctx *GroupContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitSelElement(ctx *SelElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitDsTblElement(ctx *DsTblElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitDsElement(ctx *DsElementContext) interface{} {
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
