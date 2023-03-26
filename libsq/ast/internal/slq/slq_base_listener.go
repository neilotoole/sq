// Code generated from SLQ.g4 by ANTLR 4.12.0. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr/v4"

// BaseSLQListener is a complete listener for a parse tree produced by SLQParser.
type BaseSLQListener struct{}

var _ SLQListener = &BaseSLQListener{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseSLQListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseSLQListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseSLQListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseSLQListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterStmtList is called when production stmtList is entered.
func (s *BaseSLQListener) EnterStmtList(ctx *StmtListContext) {}

// ExitStmtList is called when production stmtList is exited.
func (s *BaseSLQListener) ExitStmtList(ctx *StmtListContext) {}

// EnterQuery is called when production query is entered.
func (s *BaseSLQListener) EnterQuery(ctx *QueryContext) {}

// ExitQuery is called when production query is exited.
func (s *BaseSLQListener) ExitQuery(ctx *QueryContext) {}

// EnterSegment is called when production segment is entered.
func (s *BaseSLQListener) EnterSegment(ctx *SegmentContext) {}

// ExitSegment is called when production segment is exited.
func (s *BaseSLQListener) ExitSegment(ctx *SegmentContext) {}

// EnterElement is called when production element is entered.
func (s *BaseSLQListener) EnterElement(ctx *ElementContext) {}

// ExitElement is called when production element is exited.
func (s *BaseSLQListener) ExitElement(ctx *ElementContext) {}

// EnterCmpr is called when production cmpr is entered.
func (s *BaseSLQListener) EnterCmpr(ctx *CmprContext) {}

// ExitCmpr is called when production cmpr is exited.
func (s *BaseSLQListener) ExitCmpr(ctx *CmprContext) {}

// EnterFuncElement is called when production funcElement is entered.
func (s *BaseSLQListener) EnterFuncElement(ctx *FuncElementContext) {}

// ExitFuncElement is called when production funcElement is exited.
func (s *BaseSLQListener) ExitFuncElement(ctx *FuncElementContext) {}

// EnterFunc is called when production func is entered.
func (s *BaseSLQListener) EnterFunc(ctx *FuncContext) {}

// ExitFunc is called when production func is exited.
func (s *BaseSLQListener) ExitFunc(ctx *FuncContext) {}

// EnterFuncName is called when production funcName is entered.
func (s *BaseSLQListener) EnterFuncName(ctx *FuncNameContext) {}

// ExitFuncName is called when production funcName is exited.
func (s *BaseSLQListener) ExitFuncName(ctx *FuncNameContext) {}

// EnterJoin is called when production join is entered.
func (s *BaseSLQListener) EnterJoin(ctx *JoinContext) {}

// ExitJoin is called when production join is exited.
func (s *BaseSLQListener) ExitJoin(ctx *JoinContext) {}

// EnterJoinConstraint is called when production joinConstraint is entered.
func (s *BaseSLQListener) EnterJoinConstraint(ctx *JoinConstraintContext) {}

// ExitJoinConstraint is called when production joinConstraint is exited.
func (s *BaseSLQListener) ExitJoinConstraint(ctx *JoinConstraintContext) {}

// EnterGroupByTerm is called when production groupByTerm is entered.
func (s *BaseSLQListener) EnterGroupByTerm(ctx *GroupByTermContext) {}

// ExitGroupByTerm is called when production groupByTerm is exited.
func (s *BaseSLQListener) ExitGroupByTerm(ctx *GroupByTermContext) {}

// EnterGroupBy is called when production groupBy is entered.
func (s *BaseSLQListener) EnterGroupBy(ctx *GroupByContext) {}

// ExitGroupBy is called when production groupBy is exited.
func (s *BaseSLQListener) ExitGroupBy(ctx *GroupByContext) {}

// EnterOrderByTerm is called when production orderByTerm is entered.
func (s *BaseSLQListener) EnterOrderByTerm(ctx *OrderByTermContext) {}

// ExitOrderByTerm is called when production orderByTerm is exited.
func (s *BaseSLQListener) ExitOrderByTerm(ctx *OrderByTermContext) {}

// EnterOrderBy is called when production orderBy is entered.
func (s *BaseSLQListener) EnterOrderBy(ctx *OrderByContext) {}

// ExitOrderBy is called when production orderBy is exited.
func (s *BaseSLQListener) ExitOrderBy(ctx *OrderByContext) {}

// EnterSelector is called when production selector is entered.
func (s *BaseSLQListener) EnterSelector(ctx *SelectorContext) {}

// ExitSelector is called when production selector is exited.
func (s *BaseSLQListener) ExitSelector(ctx *SelectorContext) {}

// EnterSelectorElement is called when production selectorElement is entered.
func (s *BaseSLQListener) EnterSelectorElement(ctx *SelectorElementContext) {}

// ExitSelectorElement is called when production selectorElement is exited.
func (s *BaseSLQListener) ExitSelectorElement(ctx *SelectorElementContext) {}

// EnterAlias is called when production alias is entered.
func (s *BaseSLQListener) EnterAlias(ctx *AliasContext) {}

// ExitAlias is called when production alias is exited.
func (s *BaseSLQListener) ExitAlias(ctx *AliasContext) {}

// EnterHandleTable is called when production handleTable is entered.
func (s *BaseSLQListener) EnterHandleTable(ctx *HandleTableContext) {}

// ExitHandleTable is called when production handleTable is exited.
func (s *BaseSLQListener) ExitHandleTable(ctx *HandleTableContext) {}

// EnterHandle is called when production handle is entered.
func (s *BaseSLQListener) EnterHandle(ctx *HandleContext) {}

// ExitHandle is called when production handle is exited.
func (s *BaseSLQListener) ExitHandle(ctx *HandleContext) {}

// EnterRowRange is called when production rowRange is entered.
func (s *BaseSLQListener) EnterRowRange(ctx *RowRangeContext) {}

// ExitRowRange is called when production rowRange is exited.
func (s *BaseSLQListener) ExitRowRange(ctx *RowRangeContext) {}

// EnterExpr is called when production expr is entered.
func (s *BaseSLQListener) EnterExpr(ctx *ExprContext) {}

// ExitExpr is called when production expr is exited.
func (s *BaseSLQListener) ExitExpr(ctx *ExprContext) {}

// EnterLiteral is called when production literal is entered.
func (s *BaseSLQListener) EnterLiteral(ctx *LiteralContext) {}

// ExitLiteral is called when production literal is exited.
func (s *BaseSLQListener) ExitLiteral(ctx *LiteralContext) {}

// EnterUnaryOperator is called when production unaryOperator is entered.
func (s *BaseSLQListener) EnterUnaryOperator(ctx *UnaryOperatorContext) {}

// ExitUnaryOperator is called when production unaryOperator is exited.
func (s *BaseSLQListener) ExitUnaryOperator(ctx *UnaryOperatorContext) {}
