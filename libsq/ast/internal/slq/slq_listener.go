// Code generated from SLQ.g4 by ANTLR 4.12.0. DO NOT EDIT.

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

	// EnterFuncName is called when entering the funcName production.
	EnterFuncName(c *FuncNameContext)

	// EnterFunc is called when entering the func production.
	EnterFunc(c *FuncContext)

	// EnterFuncElement is called when entering the funcElement production.
	EnterFuncElement(c *FuncElementContext)

	// EnterJoin is called when entering the join production.
	EnterJoin(c *JoinContext)

	// EnterJoinConstraint is called when entering the joinConstraint production.
	EnterJoinConstraint(c *JoinConstraintContext)

	// EnterGroup is called when entering the group production.
	EnterGroup(c *GroupContext)

	// EnterOrderByTerm is called when entering the orderByTerm production.
	EnterOrderByTerm(c *OrderByTermContext)

	// EnterOrderBy is called when entering the orderBy production.
	EnterOrderBy(c *OrderByContext)

	// EnterSelector is called when entering the selector production.
	EnterSelector(c *SelectorContext)

	// EnterSelectorElement is called when entering the selectorElement production.
	EnterSelectorElement(c *SelectorElementContext)

	// EnterAlias is called when entering the alias production.
	EnterAlias(c *AliasContext)

	// EnterHandleTable is called when entering the handleTable production.
	EnterHandleTable(c *HandleTableContext)

	// EnterHandle is called when entering the handle production.
	EnterHandle(c *HandleContext)

	// EnterRowRange is called when entering the rowRange production.
	EnterRowRange(c *RowRangeContext)

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

	// ExitFuncName is called when exiting the funcName production.
	ExitFuncName(c *FuncNameContext)

	// ExitFunc is called when exiting the func production.
	ExitFunc(c *FuncContext)

	// ExitFuncElement is called when exiting the funcElement production.
	ExitFuncElement(c *FuncElementContext)

	// ExitJoin is called when exiting the join production.
	ExitJoin(c *JoinContext)

	// ExitJoinConstraint is called when exiting the joinConstraint production.
	ExitJoinConstraint(c *JoinConstraintContext)

	// ExitGroup is called when exiting the group production.
	ExitGroup(c *GroupContext)

	// ExitOrderByTerm is called when exiting the orderByTerm production.
	ExitOrderByTerm(c *OrderByTermContext)

	// ExitOrderBy is called when exiting the orderBy production.
	ExitOrderBy(c *OrderByContext)

	// ExitSelector is called when exiting the selector production.
	ExitSelector(c *SelectorContext)

	// ExitSelectorElement is called when exiting the selectorElement production.
	ExitSelectorElement(c *SelectorElementContext)

	// ExitAlias is called when exiting the alias production.
	ExitAlias(c *AliasContext)

	// ExitHandleTable is called when exiting the handleTable production.
	ExitHandleTable(c *HandleTableContext)

	// ExitHandle is called when exiting the handle production.
	ExitHandle(c *HandleContext)

	// ExitRowRange is called when exiting the rowRange production.
	ExitRowRange(c *RowRangeContext)

	// ExitExpr is called when exiting the expr production.
	ExitExpr(c *ExprContext)

	// ExitLiteral is called when exiting the literal production.
	ExitLiteral(c *LiteralContext)

	// ExitUnaryOperator is called when exiting the unaryOperator production.
	ExitUnaryOperator(c *UnaryOperatorContext)
}
