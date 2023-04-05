// Code generated from SLQ.g4 by ANTLR 4.12.0. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr/v4"

// A complete Visitor for a parse tree produced by SLQParser.
type SLQVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by SLQParser#stmtList.
	VisitStmtList(ctx *StmtListContext) interface{}

	// Visit a parse tree produced by SLQParser#query.
	VisitQuery(ctx *QueryContext) interface{}

	// Visit a parse tree produced by SLQParser#segment.
	VisitSegment(ctx *SegmentContext) interface{}

	// Visit a parse tree produced by SLQParser#element.
	VisitElement(ctx *ElementContext) interface{}

	// Visit a parse tree produced by SLQParser#cmpr.
	VisitCmpr(ctx *CmprContext) interface{}

	// Visit a parse tree produced by SLQParser#funcElement.
	VisitFuncElement(ctx *FuncElementContext) interface{}

	// Visit a parse tree produced by SLQParser#func.
	VisitFunc(ctx *FuncContext) interface{}

	// Visit a parse tree produced by SLQParser#funcName.
	VisitFuncName(ctx *FuncNameContext) interface{}

	// Visit a parse tree produced by SLQParser#join.
	VisitJoin(ctx *JoinContext) interface{}

	// Visit a parse tree produced by SLQParser#joinConstraint.
	VisitJoinConstraint(ctx *JoinConstraintContext) interface{}

	// Visit a parse tree produced by SLQParser#uniqueFunc.
	VisitUniqueFunc(ctx *UniqueFuncContext) interface{}

	// Visit a parse tree produced by SLQParser#countFunc.
	VisitCountFunc(ctx *CountFuncContext) interface{}

	// Visit a parse tree produced by SLQParser#groupByTerm.
	VisitGroupByTerm(ctx *GroupByTermContext) interface{}

	// Visit a parse tree produced by SLQParser#groupBy.
	VisitGroupBy(ctx *GroupByContext) interface{}

	// Visit a parse tree produced by SLQParser#orderByTerm.
	VisitOrderByTerm(ctx *OrderByTermContext) interface{}

	// Visit a parse tree produced by SLQParser#orderBy.
	VisitOrderBy(ctx *OrderByContext) interface{}

	// Visit a parse tree produced by SLQParser#selector.
	VisitSelector(ctx *SelectorContext) interface{}

	// Visit a parse tree produced by SLQParser#selectorElement.
	VisitSelectorElement(ctx *SelectorElementContext) interface{}

	// Visit a parse tree produced by SLQParser#alias.
	VisitAlias(ctx *AliasContext) interface{}

	// Visit a parse tree produced by SLQParser#arg.
	VisitArg(ctx *ArgContext) interface{}

	// Visit a parse tree produced by SLQParser#handleTable.
	VisitHandleTable(ctx *HandleTableContext) interface{}

	// Visit a parse tree produced by SLQParser#handle.
	VisitHandle(ctx *HandleContext) interface{}

	// Visit a parse tree produced by SLQParser#rowRange.
	VisitRowRange(ctx *RowRangeContext) interface{}

	// Visit a parse tree produced by SLQParser#expr.
	VisitExpr(ctx *ExprContext) interface{}

	// Visit a parse tree produced by SLQParser#literal.
	VisitLiteral(ctx *LiteralContext) interface{}

	// Visit a parse tree produced by SLQParser#unaryOperator.
	VisitUnaryOperator(ctx *UnaryOperatorContext) interface{}
}
