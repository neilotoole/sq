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

func (v *BaseSLQVisitor) VisitFuncElement(ctx *FuncElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFunc(ctx *FuncContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitFuncName(ctx *FuncNameContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitJoin(ctx *JoinContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitJoinConstraint(ctx *JoinConstraintContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitUniqueFunc(ctx *UniqueFuncContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitCountFunc(ctx *CountFuncContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitGroupByTerm(ctx *GroupByTermContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitGroupBy(ctx *GroupByContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitOrderByTerm(ctx *OrderByTermContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitOrderBy(ctx *OrderByContext) interface{} {
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

func (v *BaseSLQVisitor) VisitExpr(ctx *ExprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitLiteral(ctx *LiteralContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *BaseSLQVisitor) VisitUnaryOperator(ctx *UnaryOperatorContext) interface{} {
	return v.VisitChildren(ctx)
}
