// Code generated from java-escape by ANTLR 4.11.1. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr/v4"

// SLQListener is a complete listener for a parse tree produced by SLQParser.
type SLQListener interface {
	antlr.ParseTreeListener

	// EnterStmtList is called when entering the stmtList production.
	EnterStmtList(c *StmtListContext)

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

	// EnterJoin is called when entering the join production.
	EnterJoin(c *JoinContext)

	// EnterJoinConstraint is called when entering the joinConstraint production.
	EnterJoinConstraint(c *JoinConstraintContext)

	// EnterGroup is called when entering the group production.
	EnterGroup(c *GroupContext)

	// EnterSelElement is called when entering the selElement production.
	EnterSelElement(c *SelElementContext)

	// EnterDsTblElement is called when entering the dsTblElement production.
	EnterDsTblElement(c *DsTblElementContext)

	// EnterDsElement is called when entering the dsElement production.
	EnterDsElement(c *DsElementContext)

	// EnterRowRange is called when entering the rowRange production.
	EnterRowRange(c *RowRangeContext)

	// EnterFnName is called when entering the fnName production.
	EnterFnName(c *FnNameContext)

	// EnterExpr is called when entering the expr production.
	EnterExpr(c *ExprContext)

	// EnterLiteral is called when entering the literal production.
	EnterLiteral(c *LiteralContext)

	// EnterUnaryOperator is called when entering the unaryOperator production.
	EnterUnaryOperator(c *UnaryOperatorContext)

	// ExitStmtList is called when exiting the stmtList production.
	ExitStmtList(c *StmtListContext)

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

	// ExitJoin is called when exiting the join production.
	ExitJoin(c *JoinContext)

	// ExitJoinConstraint is called when exiting the joinConstraint production.
	ExitJoinConstraint(c *JoinConstraintContext)

	// ExitGroup is called when exiting the group production.
	ExitGroup(c *GroupContext)

	// ExitSelElement is called when exiting the selElement production.
	ExitSelElement(c *SelElementContext)

	// ExitDsTblElement is called when exiting the dsTblElement production.
	ExitDsTblElement(c *DsTblElementContext)

	// ExitDsElement is called when exiting the dsElement production.
	ExitDsElement(c *DsElementContext)

	// ExitRowRange is called when exiting the rowRange production.
	ExitRowRange(c *RowRangeContext)

	// ExitFnName is called when exiting the fnName production.
	ExitFnName(c *FnNameContext)

	// ExitExpr is called when exiting the expr production.
	ExitExpr(c *ExprContext)

	// ExitLiteral is called when exiting the literal production.
	ExitLiteral(c *LiteralContext)

	// ExitUnaryOperator is called when exiting the unaryOperator production.
	ExitUnaryOperator(c *UnaryOperatorContext)
}
