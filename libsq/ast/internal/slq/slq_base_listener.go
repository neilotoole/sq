// Code generated from SLQ.g4 by ANTLR 4.13.0. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr4-go/antlr/v4"

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

// EnterJoinTable is called when production joinTable is entered.
func (s *BaseSLQListener) EnterJoinTable(ctx *JoinTableContext) {}

// ExitJoinTable is called when production joinTable is exited.
func (s *BaseSLQListener) ExitJoinTable(ctx *JoinTableContext) {}

// EnterUniqueFunc is called when production uniqueFunc is entered.
func (s *BaseSLQListener) EnterUniqueFunc(ctx *UniqueFuncContext) {}

// ExitUniqueFunc is called when production uniqueFunc is exited.
func (s *BaseSLQListener) ExitUniqueFunc(ctx *UniqueFuncContext) {}

// EnterCountFunc is called when production countFunc is entered.
func (s *BaseSLQListener) EnterCountFunc(ctx *CountFuncContext) {}

// ExitCountFunc is called when production countFunc is exited.
func (s *BaseSLQListener) ExitCountFunc(ctx *CountFuncContext) {}

// EnterWhere is called when production where is entered.
func (s *BaseSLQListener) EnterWhere(ctx *WhereContext) {}

// ExitWhere is called when production where is exited.
func (s *BaseSLQListener) ExitWhere(ctx *WhereContext) {}

// EnterGroupByTerm is called when production groupByTerm is entered.
func (s *BaseSLQListener) EnterGroupByTerm(ctx *GroupByTermContext) {}

// ExitGroupByTerm is called when production groupByTerm is exited.
func (s *BaseSLQListener) ExitGroupByTerm(ctx *GroupByTermContext) {}

// EnterGroupBy is called when production groupBy is entered.
func (s *BaseSLQListener) EnterGroupBy(ctx *GroupByContext) {}

// ExitGroupBy is called when production groupBy is exited.
func (s *BaseSLQListener) ExitGroupBy(ctx *GroupByContext) {}

// EnterHaving is called when production having is entered.
func (s *BaseSLQListener) EnterHaving(ctx *HavingContext) {}

// ExitHaving is called when production having is exited.
func (s *BaseSLQListener) ExitHaving(ctx *HavingContext) {}

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

// EnterArg is called when production arg is entered.
func (s *BaseSLQListener) EnterArg(ctx *ArgContext) {}

// ExitArg is called when production arg is exited.
func (s *BaseSLQListener) ExitArg(ctx *ArgContext) {}

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

// EnterExprElement is called when production exprElement is entered.
func (s *BaseSLQListener) EnterExprElement(ctx *ExprElementContext) {}

// ExitExprElement is called when production exprElement is exited.
func (s *BaseSLQListener) ExitExprElement(ctx *ExprElementContext) {}

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
