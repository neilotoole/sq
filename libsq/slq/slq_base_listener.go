// Generated from ../grammar/SLQ.g4 by ANTLR 4.5.3.

package slq // SLQ
import "github.com/pboyer/antlr4/runtime/Go/antlr"

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

// EnterArgs is called when production args is entered.
func (s *BaseSLQListener) EnterArgs(ctx *ArgsContext) {}

// ExitArgs is called when production args is exited.
func (s *BaseSLQListener) ExitArgs(ctx *ArgsContext) {}

// EnterArg is called when production arg is entered.
func (s *BaseSLQListener) EnterArg(ctx *ArgContext) {}

// ExitArg is called when production arg is exited.
func (s *BaseSLQListener) ExitArg(ctx *ArgContext) {}

// EnterJoin is called when production join is entered.
func (s *BaseSLQListener) EnterJoin(ctx *JoinContext) {}

// ExitJoin is called when production join is exited.
func (s *BaseSLQListener) ExitJoin(ctx *JoinContext) {}

// EnterJoinConstraint is called when production joinConstraint is entered.
func (s *BaseSLQListener) EnterJoinConstraint(ctx *JoinConstraintContext) {}

// ExitJoinConstraint is called when production joinConstraint is exited.
func (s *BaseSLQListener) ExitJoinConstraint(ctx *JoinConstraintContext) {}

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
