// Generated from ../grammar/SQ.g4 by ANTLR 4.5.1.

package parser // SQ
import "github.com/pboyer/antlr4/runtime/Go/antlr"

// A complete Visitor for a parse tree produced by SQParser.
type SQVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by SQParser#query.
	VisitQuery(ctx *QueryContext) interface{}

	// Visit a parse tree produced by SQParser#segment.
	VisitSegment(ctx *SegmentContext) interface{}

	// Visit a parse tree produced by SQParser#element.
	VisitElement(ctx *ElementContext) interface{}

	// Visit a parse tree produced by SQParser#cmpr.
	VisitCmpr(ctx *CmprContext) interface{}

	// Visit a parse tree produced by SQParser#fn.
	VisitFn(ctx *FnContext) interface{}

	// Visit a parse tree produced by SQParser#args.
	VisitArgs(ctx *ArgsContext) interface{}

	// Visit a parse tree produced by SQParser#arg.
	VisitArg(ctx *ArgContext) interface{}

	// Visit a parse tree produced by SQParser#fnJoin.
	VisitFnJoin(ctx *FnJoinContext) interface{}

	// Visit a parse tree produced by SQParser#fnJoinCond.
	VisitFnJoinCond(ctx *FnJoinCondContext) interface{}

	// Visit a parse tree produced by SQParser#fnJoinExpr.
	VisitFnJoinExpr(ctx *FnJoinExprContext) interface{}

	// Visit a parse tree produced by SQParser#selElement.
	VisitSelElement(ctx *SelElementContext) interface{}

	// Visit a parse tree produced by SQParser#dsTblElement.
	VisitDsTblElement(ctx *DsTblElementContext) interface{}

	// Visit a parse tree produced by SQParser#dsElement.
	VisitDsElement(ctx *DsElementContext) interface{}

	// Visit a parse tree produced by SQParser#rowRange.
	VisitRowRange(ctx *RowRangeContext) interface{}
}
