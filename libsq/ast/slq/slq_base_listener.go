// Code generated from /Users/neilotoole/work/sq/sq/grammar/SLQ.g4 by ANTLR 4.7.2. DO NOT EDIT.

package slq // SLQ
import "github.com/antlr/antlr4/runtime/Go/antlr"

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

// EnterFn is called when production fn is entered.
func (s *BaseSLQListener) EnterFn(ctx *FnContext) {}

// ExitFn is called when production fn is exited.
func (s *BaseSLQListener) ExitFn(ctx *FnContext) {}

// EnterJoin is called when production join is entered.
func (s *BaseSLQListener) EnterJoin(ctx *JoinContext) {}

// ExitJoin is called when production join is exited.
func (s *BaseSLQListener) ExitJoin(ctx *JoinContext) {}

// EnterJoinConstraint is called when production joinConstraint is entered.
func (s *BaseSLQListener) EnterJoinConstraint(ctx *JoinConstraintContext) {}

// ExitJoinConstraint is called when production joinConstraint is exited.
func (s *BaseSLQListener) ExitJoinConstraint(ctx *JoinConstraintContext) {}

// EnterGroup is called when production group is entered.
func (s *BaseSLQListener) EnterGroup(ctx *GroupContext) {}

// ExitGroup is called when production group is exited.
func (s *BaseSLQListener) ExitGroup(ctx *GroupContext) {}

// EnterSelElement is called when production selElement is entered.
func (s *BaseSLQListener) EnterSelElement(ctx *SelElementContext) {}

// ExitSelElement is called when production selElement is exited.
func (s *BaseSLQListener) ExitSelElement(ctx *SelElementContext) {}

// EnterDsTblElement is called when production dsTblElement is entered.
func (s *BaseSLQListener) EnterDsTblElement(ctx *DsTblElementContext) {}

// ExitDsTblElement is called when production dsTblElement is exited.
func (s *BaseSLQListener) ExitDsTblElement(ctx *DsTblElementContext) {}

// EnterDsElement is called when production dsElement is entered.
func (s *BaseSLQListener) EnterDsElement(ctx *DsElementContext) {}

// ExitDsElement is called when production dsElement is exited.
func (s *BaseSLQListener) ExitDsElement(ctx *DsElementContext) {}

// EnterRowRange is called when production rowRange is entered.
func (s *BaseSLQListener) EnterRowRange(ctx *RowRangeContext) {}

// ExitRowRange is called when production rowRange is exited.
func (s *BaseSLQListener) ExitRowRange(ctx *RowRangeContext) {}

// EnterFnName is called when production fnName is entered.
func (s *BaseSLQListener) EnterFnName(ctx *FnNameContext) {}

// ExitFnName is called when production fnName is exited.
func (s *BaseSLQListener) ExitFnName(ctx *FnNameContext) {}

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
