// Generated from ../grammar/SLQ.g4 by ANTLR 4.5.1.

package slq // SLQ
import "github.com/pboyer/antlr4/runtime/Go/antlr"

type BaseSLQVisitor struct {
	*antlr.BaseParseTreeVisitor
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

func (v *BaseSLQVisitor) VisitArgs(ctx *ArgsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitArg(ctx *ArgContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFnJoin(ctx *FnJoinContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFnJoinCond(ctx *FnJoinCondContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFnJoinExpr(ctx *FnJoinExprContext) interface{} {
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
