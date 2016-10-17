// Generated from ../grammar/SLQ.g4 by ANTLR 4.5.1.

package slq // SLQ
import "github.com/pboyer/antlr4/runtime/Go/antlr"

// SLQListener is a complete listener for a parse tree produced by SLQParser.
type SLQListener interface {
	antlr.ParseTreeListener

	// EnterQuery is called when entering the query production.
	EnterQuery(c *QueryContext)

	// EnterSegment is called when entering the segment production.
	EnterSegment(c *SegmentContext)

	// EnterElement is called when entering the element production.
	EnterElement(c *ElementContext)

	// EnterCmpr is called when entering the cmpr production.
	EnterCmpr(c *CmprContext)

	// EnterFn is called when entering the fn production.
	EnterFn(c *FnContext)

	// EnterArgs is called when entering the args production.
	EnterArgs(c *ArgsContext)

	// EnterArg is called when entering the arg production.
	EnterArg(c *ArgContext)

	// EnterFnJoin is called when entering the fnJoin production.
	EnterFnJoin(c *FnJoinContext)

	// EnterFnJoinCond is called when entering the fnJoinCond production.
	EnterFnJoinCond(c *FnJoinCondContext)

	// EnterFnJoinExpr is called when entering the fnJoinExpr production.
	EnterFnJoinExpr(c *FnJoinExprContext)

	// EnterSelElement is called when entering the selElement production.
	EnterSelElement(c *SelElementContext)

	// EnterDsTblElement is called when entering the dsTblElement production.
	EnterDsTblElement(c *DsTblElementContext)

	// EnterDsElement is called when entering the dsElement production.
	EnterDsElement(c *DsElementContext)

	// EnterRowRange is called when entering the rowRange production.
	EnterRowRange(c *RowRangeContext)

	// ExitQuery is called when exiting the query production.
	ExitQuery(c *QueryContext)

	// ExitSegment is called when exiting the segment production.
	ExitSegment(c *SegmentContext)

	// ExitElement is called when exiting the element production.
	ExitElement(c *ElementContext)

	// ExitCmpr is called when exiting the cmpr production.
	ExitCmpr(c *CmprContext)

	// ExitFn is called when exiting the fn production.
	ExitFn(c *FnContext)

	// ExitArgs is called when exiting the args production.
	ExitArgs(c *ArgsContext)

	// ExitArg is called when exiting the arg production.
	ExitArg(c *ArgContext)

	// ExitFnJoin is called when exiting the fnJoin production.
	ExitFnJoin(c *FnJoinContext)

	// ExitFnJoinCond is called when exiting the fnJoinCond production.
	ExitFnJoinCond(c *FnJoinCondContext)

	// ExitFnJoinExpr is called when exiting the fnJoinExpr production.
	ExitFnJoinExpr(c *FnJoinExprContext)

	// ExitSelElement is called when exiting the selElement production.
	ExitSelElement(c *SelElementContext)

	// ExitDsTblElement is called when exiting the dsTblElement production.
	ExitDsTblElement(c *DsTblElementContext)

	// ExitDsElement is called when exiting the dsElement production.
	ExitDsElement(c *DsElementContext)

	// ExitRowRange is called when exiting the rowRange production.
	ExitRowRange(c *RowRangeContext)
}
