// Code generated from /Users/neilotoole/work/moi/go/src/github.com/neilotoole/sq/grammar/SLQ.g4 by ANTLR 4.7.2. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr"

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

	// Visit a parse tree produced by SLQParser#fn.
	VisitFn(ctx *FnContext) interface{}

	// Visit a parse tree produced by SLQParser#join.
	VisitJoin(ctx *JoinContext) interface{}

	// Visit a parse tree produced by SLQParser#joinConstraint.
	VisitJoinConstraint(ctx *JoinConstraintContext) interface{}

	// Visit a parse tree produced by SLQParser#group.
	VisitGroup(ctx *GroupContext) interface{}

	// Visit a parse tree produced by SLQParser#selElement.
	VisitSelElement(ctx *SelElementContext) interface{}

	// Visit a parse tree produced by SLQParser#dsTblElement.
	VisitDsTblElement(ctx *DsTblElementContext) interface{}

	// Visit a parse tree produced by SLQParser#dsElement.
	VisitDsElement(ctx *DsElementContext) interface{}

	// Visit a parse tree produced by SLQParser#rowRange.
	VisitRowRange(ctx *RowRangeContext) interface{}

	// Visit a parse tree produced by SLQParser#fnName.
	VisitFnName(ctx *FnNameContext) interface{}

	// Visit a parse tree produced by SLQParser#expr.
	VisitExpr(ctx *ExprContext) interface{}

	// Visit a parse tree produced by SLQParser#literal.
	VisitLiteral(ctx *LiteralContext) interface{}

	// Visit a parse tree produced by SLQParser#unaryOperator.
	VisitUnaryOperator(ctx *UnaryOperatorContext) interface{}
}
