// Generated from ../grammar/SQ.g4 by ANTLR 4.5.1.

package parser // SQ
import "github.com/pboyer/antlr4/runtime/Go/antlr"

type BaseSQVisitor struct {
	*antlr.BaseParseTreeVisitor
}

func (v *BaseSQVisitor) VisitQuery(ctx *QueryContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitSegment(ctx *SegmentContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitElement(ctx *ElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitCmpr(ctx *CmprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitFn(ctx *FnContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitArgs(ctx *ArgsContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitArg(ctx *ArgContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitFnJoin(ctx *FnJoinContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitFnJoinCond(ctx *FnJoinCondContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitFnJoinExpr(ctx *FnJoinExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitSelElement(ctx *SelElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitDsTblElement(ctx *DsTblElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitDsElement(ctx *DsElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSQVisitor) VisitRowRange(ctx *RowRangeContext) interface{} {
	return v.VisitChildren(ctx)
}
