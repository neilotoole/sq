// Generated from ../grammar/SLQ.g4 by ANTLR 4.5.3.

package slq // SLQ
import "github.com/pboyer/antlr4/runtime/Go/antlr"

// A complete Visitor for a parse tree produced by SLQParser.
type SLQVisitor interface {
	antlr.ParseTreeVisitor

	// Visit a parse tree produced by SLQParser#query.
	VisitQuery(ctx *QueryContext) interface{}

	// Visit a parse tree produced by SLQParser#segment.
	VisitSegment(ctx *SegmentContext) interface{}

	// Visit a parse tree produced by SLQParser#element.
	VisitElement(ctx *ElementContext) interface{}

	// Visit a parse tree produced by SLQParser#cmpr.
	VisitCmpr(ctx *CmprContext) interface{}

	// Visit a parse tree produced by SLQParser#args.
	VisitArgs(ctx *ArgsContext) interface{}

	// Visit a parse tree produced by SLQParser#arg.
	VisitArg(ctx *ArgContext) interface{}

	// Visit a parse tree produced by SLQParser#join.
	VisitJoin(ctx *JoinContext) interface{}

	// Visit a parse tree produced by SLQParser#joinConstraint.
	VisitJoinConstraint(ctx *JoinConstraintContext) interface{}

	// Visit a parse tree produced by SLQParser#selElement.
	VisitSelElement(ctx *SelElementContext) interface{}

	// Visit a parse tree produced by SLQParser#dsTblElement.
	VisitDsTblElement(ctx *DsTblElementContext) interface{}

	// Visit a parse tree produced by SLQParser#dsElement.
	VisitDsElement(ctx *DsElementContext) interface{}

	// Visit a parse tree produced by SLQParser#rowRange.
	VisitRowRange(ctx *RowRangeContext) interface{}
}
