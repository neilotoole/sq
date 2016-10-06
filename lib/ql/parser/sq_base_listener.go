// Generated from ../grammar/SQ.g4 by ANTLR 4.5.1.

package parser // SQ
import "github.com/pboyer/antlr4/runtime/Go/antlr"

// BaseSQListener is a complete listener for a parse tree produced by SQParser.
type BaseSQListener struct{}

// VisitTerminal is called when a terminal node is visited.
func (s *BaseSQListener) VisitTerminal(node antlr.TerminalNode) {}

// VisitErrorNode is called when an error node is visited.
func (s *BaseSQListener) VisitErrorNode(node antlr.ErrorNode) {}

// EnterEveryRule is called when any rule is entered.
func (s *BaseSQListener) EnterEveryRule(ctx antlr.ParserRuleContext) {}

// ExitEveryRule is called when any rule is exited.
func (s *BaseSQListener) ExitEveryRule(ctx antlr.ParserRuleContext) {}

// EnterQuery is called when production query is entered.
func (s *BaseSQListener) EnterQuery(ctx *QueryContext) {}

// ExitQuery is called when production query is exited.
func (s *BaseSQListener) ExitQuery(ctx *QueryContext) {}

// EnterSegment is called when production segment is entered.
func (s *BaseSQListener) EnterSegment(ctx *SegmentContext) {}

// ExitSegment is called when production segment is exited.
func (s *BaseSQListener) ExitSegment(ctx *SegmentContext) {}

// EnterElement is called when production element is entered.
func (s *BaseSQListener) EnterElement(ctx *ElementContext) {}

// ExitElement is called when production element is exited.
func (s *BaseSQListener) ExitElement(ctx *ElementContext) {}

// EnterCmpr is called when production cmpr is entered.
func (s *BaseSQListener) EnterCmpr(ctx *CmprContext) {}

// ExitCmpr is called when production cmpr is exited.
func (s *BaseSQListener) ExitCmpr(ctx *CmprContext) {}

// EnterFn is called when production fn is entered.
func (s *BaseSQListener) EnterFn(ctx *FnContext) {}

// ExitFn is called when production fn is exited.
func (s *BaseSQListener) ExitFn(ctx *FnContext) {}

// EnterArgs is called when production args is entered.
func (s *BaseSQListener) EnterArgs(ctx *ArgsContext) {}

// ExitArgs is called when production args is exited.
func (s *BaseSQListener) ExitArgs(ctx *ArgsContext) {}

// EnterArg is called when production arg is entered.
func (s *BaseSQListener) EnterArg(ctx *ArgContext) {}

// ExitArg is called when production arg is exited.
func (s *BaseSQListener) ExitArg(ctx *ArgContext) {}

// EnterFnJoin is called when production fnJoin is entered.
func (s *BaseSQListener) EnterFnJoin(ctx *FnJoinContext) {}

// ExitFnJoin is called when production fnJoin is exited.
func (s *BaseSQListener) ExitFnJoin(ctx *FnJoinContext) {}

// EnterFnJoinCond is called when production fnJoinCond is entered.
func (s *BaseSQListener) EnterFnJoinCond(ctx *FnJoinCondContext) {}

// ExitFnJoinCond is called when production fnJoinCond is exited.
func (s *BaseSQListener) ExitFnJoinCond(ctx *FnJoinCondContext) {}

// EnterFnJoinExpr is called when production fnJoinExpr is entered.
func (s *BaseSQListener) EnterFnJoinExpr(ctx *FnJoinExprContext) {}

// ExitFnJoinExpr is called when production fnJoinExpr is exited.
func (s *BaseSQListener) ExitFnJoinExpr(ctx *FnJoinExprContext) {}

// EnterSelElement is called when production selElement is entered.
func (s *BaseSQListener) EnterSelElement(ctx *SelElementContext) {}

// ExitSelElement is called when production selElement is exited.
func (s *BaseSQListener) ExitSelElement(ctx *SelElementContext) {}

// EnterDsTblElement is called when production dsTblElement is entered.
func (s *BaseSQListener) EnterDsTblElement(ctx *DsTblElementContext) {}

// ExitDsTblElement is called when production dsTblElement is exited.
func (s *BaseSQListener) ExitDsTblElement(ctx *DsTblElementContext) {}

// EnterDsElement is called when production dsElement is entered.
func (s *BaseSQListener) EnterDsElement(ctx *DsElementContext) {}

// ExitDsElement is called when production dsElement is exited.
func (s *BaseSQListener) ExitDsElement(ctx *DsElementContext) {}

// EnterRowRange is called when production rowRange is entered.
func (s *BaseSQListener) EnterRowRange(ctx *RowRangeContext) {}

// ExitRowRange is called when production rowRange is exited.
func (s *BaseSQListener) ExitRowRange(ctx *RowRangeContext) {}
