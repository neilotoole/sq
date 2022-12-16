// Code generated from /Users/neilotoole/work/sq/sq/grammar/SLQ.g4 by ANTLR 4.7.2. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr"

// A complete Visitor for a parse tree produced by SLQParser.
type SLQVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by SLQParser#stmtList.
	VisitStmtList(ctx *StmtListContext) any

	// Visit a parse tree produced by SLQParser#query.
	VisitQuery(ctx *QueryContext) any

	// Visit a parse tree produced by SLQParser#segment.
	VisitSegment(ctx *SegmentContext) any

	// Visit a parse tree produced by SLQParser#element.
	VisitElement(ctx *ElementContext) any

	// Visit a parse tree produced by SLQParser#cmpr.
	VisitCmpr(ctx *CmprContext) any

	// Visit a parse tree produced by SLQParser#fn.
	VisitFn(ctx *FnContext) any

	// Visit a parse tree produced by SLQParser#join.
	VisitJoin(ctx *JoinContext) any

	// Visit a parse tree produced by SLQParser#joinConstraint.
	VisitJoinConstraint(ctx *JoinConstraintContext) any

	// Visit a parse tree produced by SLQParser#group.
	VisitGroup(ctx *GroupContext) any

	// Visit a parse tree produced by SLQParser#selElement.
	VisitSelElement(ctx *SelElementContext) any

	// Visit a parse tree produced by SLQParser#dsTblElement.
	VisitDsTblElement(ctx *DsTblElementContext) any

	// Visit a parse tree produced by SLQParser#dsElement.
	VisitDsElement(ctx *DsElementContext) any

	// Visit a parse tree produced by SLQParser#rowRange.
	VisitRowRange(ctx *RowRangeContext) any

	// Visit a parse tree produced by SLQParser#fnName.
	VisitFnName(ctx *FnNameContext) any

	// Visit a parse tree produced by SLQParser#expr.
	VisitExpr(ctx *ExprContext) any

	// Visit a parse tree produced by SLQParser#literal.
	VisitLiteral(ctx *LiteralContext) any

	// Visit a parse tree produced by SLQParser#unaryOperator.
	VisitUnaryOperator(ctx *UnaryOperatorContext) any
}
