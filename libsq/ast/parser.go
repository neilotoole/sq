package ast

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// parseSLQ processes SLQ input text according to the rules of the SQL grammar,
// and returns a parse tree. It executes both lexer and parser phases.
func parseSLQ(log lg.Log, input string) (*slq.QueryContext, error) {
	lex := slq.NewSLQLexer(antlr.NewInputStream(input))
	lex.RemoveErrorListeners() // the generated lexer has default listeners we don't want
	lexErrs := &antlrErrorListener{name: "lexer", log: log}
	lex.AddErrorListener(lexErrs)

	p := slq.NewSLQParser(antlr.NewCommonTokenStream(lex, 0))
	p.RemoveErrorListeners() // the generated parser has default listeners we don't want
	parseErrs := &antlrErrorListener{name: "parser", log: log}
	p.AddErrorListener(parseErrs)

	qCtx := p.Query()
	err := lexErrs.error()
	if err != nil {
		return nil, err
	}

	err = parseErrs.error()
	if err != nil {
		return nil, err
	}

	return qCtx.(*slq.QueryContext), nil
}

type antlrErrorListener struct {
	log      lg.Log
	name     string
	errs     []string
	warnings []string
	err      error
}

func (el *antlrErrorListener) error() error {
	if el.err == nil && len(el.errs) > 0 {
		msg := strings.Join(el.errs, "\n")
		el.err = &parseError{msg: msg}
	}
	return el.err
}

func (el *antlrErrorListener) String() string {
	if len(el.errs)+len(el.warnings) == 0 {
		return fmt.Sprintf("%s: no issues", el.name)
	}

	strs := make([]string, 0, len(el.errs)+len(el.warnings))
	strs = append(strs, el.errs...)
	strs = append(strs, el.warnings...)

	return strings.Join(strs, "\n")
}

// SyntaxError implements antlr.ErrorListener.
func (el *antlrErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol any, line, column int,
	msg string, e antlr.RecognitionException,
) {
	text := fmt.Sprintf("%s: syntax error: [%d:%d] %s", el.name, line, column, msg)
	el.errs = append(el.errs, text)
}

// ReportAmbiguity implements antlr.ErrorListener.
func (el *antlrErrorListener) ReportAmbiguity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int,
	exact bool, ambigAlts *antlr.BitSet, configs antlr.ATNConfigSet,
) {
	tok := recognizer.GetCurrentToken()
	text := fmt.Sprintf("%s: syntax ambiguity: [%d:%d]", el.name, startIndex, stopIndex)
	text = text + "  >>" + tok.GetText() + "<<"
	el.warnings = append(el.warnings, text)
}

// ReportAttemptingFullContext implements antlr.ErrorListener.
func (el *antlrErrorListener) ReportAttemptingFullContext(recognizer antlr.Parser, dfa *antlr.DFA, startIndex,
	stopIndex int, conflictingAlts *antlr.BitSet, configs antlr.ATNConfigSet,
) {
	text := fmt.Sprintf("%s: attempting full context: [%d:%d]", el.name, startIndex, stopIndex)
	el.warnings = append(el.warnings, text)
}

// ReportContextSensitivity implements antlr.ErrorListener.
func (el *antlrErrorListener) ReportContextSensitivity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex,
	prediction int, configs antlr.ATNConfigSet,
) {
	text := fmt.Sprintf("%s: context sensitivity: [%d:%d]", el.name, startIndex, stopIndex)
	el.warnings = append(el.warnings, text)
}

// parseError represents an error in lexing/parsing input.
type parseError struct {
	msg string
	// TODO: parse error should include more detail, such as the offending token, position, etc.
}

func (p *parseError) Error() string {
	return p.msg
}

var _ slq.SLQVisitor = (*parseTreeVisitor)(nil)

// parseTreeVisitor implements slq.SLQVisitor to
// generate the preliminary AST.
type parseTreeVisitor struct {
	log lg.Log

	// cur is the currently-active node of the AST.
	cur Node

	AST *AST
}

// using is a convenience function that sets v.cur to cur,
// executes fn, and then restores v.cur to its previous value.
// The type of the returned value is declared as "any" instead of
// error, because that's the generated antlr code returns "any".
func (v *parseTreeVisitor) using(cur Node, fn func() any) any {
	prev := v.cur
	v.cur = cur
	defer func() { v.cur = prev }()
	return fn()
}

// Visit implements antlr.ParseTreeVisitor.
func (v *parseTreeVisitor) Visit(ctx antlr.ParseTree) any {
	v.log.Debugf("visiting %T: %v", ctx, ctx.GetText())

	switch ctx := ctx.(type) {
	case *slq.SegmentContext:
		return v.VisitSegment(ctx)
	case *slq.ElementContext:
		return v.VisitElement(ctx)
	case *slq.HandleElementContext:
		return v.VisitHandleElement(ctx)
	case *slq.DsTblElementContext:
		return v.VisitDsTblElement(ctx)
	case *slq.SelElementContext:
		return v.VisitSelElement(ctx)
	case *slq.FnElementContext:
		return v.VisitFnElement(ctx)
	case *slq.FnContext:
		return v.VisitFn(ctx)
	case *slq.FnNameContext:
		return v.VisitFnName(ctx)
	case *slq.JoinContext:
		return v.VisitJoin(ctx)
	case *slq.AliasContext:
		return v.VisitAlias(ctx)
	case *slq.JoinConstraintContext:
		return v.VisitJoinConstraint(ctx)
	case *slq.CmprContext:
		return v.VisitCmpr(ctx)
	case *slq.RowRangeContext:
		return v.VisitRowRange(ctx)
	case *slq.ExprContext:
		return v.VisitExpr(ctx)
	case *slq.GroupContext:
		return v.VisitGroup(ctx)
	case *slq.LiteralContext:
		return v.VisitLiteral(ctx)
	case *antlr.TerminalNodeImpl:
		return v.VisitTerminal(ctx)
	}

	// should never be reached
	return errorf("unknown node type: %T", ctx)
}

// VisitChildren implements antlr.ParseTreeVisitor.
func (v *parseTreeVisitor) VisitChildren(ctx antlr.RuleNode) any {
	for _, child := range ctx.GetChildren() {
		tree, ok := child.(antlr.ParseTree)
		if !ok {
			return errorf("unknown child node type: %T %q", child, child.GetPayload())
		}

		err := v.Visit(tree)
		if err != nil {
			return err
		}
	}
	return nil
}

// VisitQuery implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitQuery(ctx *slq.QueryContext) any {
	v.AST = &AST{}
	v.AST.ctx = ctx
	v.cur = v.AST

	for _, seg := range ctx.AllSegment() {
		err := v.VisitSegment(seg.(*slq.SegmentContext))
		if err != nil {
			return err
		}
	}

	return nil
}

// VisitHandleElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitHandleElement(ctx *slq.HandleElementContext) any {
	ds := &Datasource{}
	ds.parent = v.cur
	ds.ctx = ctx.HANDLE()
	return v.cur.AddChild(ds)
}

// VisitDsTblElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitDsTblElement(ctx *slq.DsTblElementContext) any {
	tblSel := &TblSelector{}
	tblSel.parent = v.cur
	tblSel.ctx = ctx

	tblSel.DSName = ctx.HANDLE().GetText()
	tblSel.TblName = ctx.SEL().GetText()[1:]

	return v.cur.AddChild(tblSel)
}

// VisitSegment implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSegment(ctx *slq.SegmentContext) any {
	seg := &Segment{}
	seg.bn.ctx = ctx
	seg.bn.parent = v.AST

	v.AST.AddSegment(seg)
	v.cur = seg

	return v.VisitChildren(ctx)
}

// VisitSelElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSelElement(ctx *slq.SelElementContext) any {
	selector := &Selector{}
	selector.parent = v.cur
	selector.ctx = ctx.SEL()

	var err any
	if err = v.cur.AddChild(selector); err != nil {
		return err
	}

	return v.using(selector, func() any {
		return v.VisitChildren(ctx)
	})
}

// VisitElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitElement(ctx *slq.ElementContext) any {
	return v.VisitChildren(ctx)
}

// VisitAlias implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitAlias(ctx *slq.AliasContext) any {
	alias := ctx.ID().GetText()

	switch node := v.cur.(type) {
	case *Selector:
		node.alias = alias
	case *Func:
		node.alias = alias
	default:
		return errorf("alias not allowed for type %T: %v", node, ctx.GetText())
	}

	return nil
}

// VisitFnElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitFnElement(ctx *slq.FnElementContext) any {
	v.log.Debugf("visiting FnElement: %v", ctx.GetText())

	childCount := ctx.GetChildCount()
	if childCount == 0 || childCount > 2 {
		return errorf("parser: invalid function: expected 1 or 2 children, but got %d: %v",
			childCount, ctx.GetText())
	}

	// e.g. count(*)
	child1 := ctx.GetChild(0)
	fnCtx, ok := child1.(*slq.FnContext)
	if !ok {
		return errorf("expected first child to be %T but was %T: %v", fnCtx, child1, ctx.GetText())
	}

	if err := v.VisitFn(fnCtx); err != nil {
		return err
	}

	// Check if there's an alias
	if childCount == 2 {
		child2 := ctx.GetChild(1)
		aliasCtx, ok := child2.(*slq.AliasContext)
		if !ok {
			return errorf("expected second child to be %T but was %T: %v", aliasCtx, child2, ctx.GetText())
		}

		// VisitAlias will expect v.cur to be a Func.
		lastNode := nodeLastChild(v.cur)
		fnNode, ok := lastNode.(*Func)
		if !ok {
			return errorf("expected %T but got %T: %v", fnNode, lastNode, ctx.GetText())
		}

		return v.using(fnNode, func() any {
			return v.VisitAlias(aliasCtx)
		})
	}

	return nil
}

// VisitFn implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitFn(ctx *slq.FnContext) any {
	v.log.Debugf("visiting Fn: %v", ctx.GetText())

	fn := &Func{fnName: ctx.FnName().GetText()}
	fn.ctx = ctx
	err := fn.SetParent(v.cur)
	if err != nil {
		return err
	}

	if err2 := v.using(fn, func() any {
		return v.VisitChildren(ctx)
	}); err2 != nil {
		return err2
	}

	return v.cur.AddChild(fn)
}

// VisitExpr implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitExpr(ctx *slq.ExprContext) any {
	v.log.Debugf("visiting expr: %v", ctx.GetText())

	// check if the expr is a SEL, e.g. ".uid"
	if ctx.SEL() != nil {
		selector := &Selector{}
		selector.parent = v.cur
		selector.ctx = ctx.SEL()
		return v.cur.AddChild(selector)
	}

	if ctx.Literal() != nil {
		return v.VisitLiteral(ctx.Literal().(*slq.LiteralContext))
	}

	ex := &Expr{}
	ex.ctx = ctx
	err := ex.SetParent(v.cur)
	if err != nil {
		return err
	}

	prev := v.cur
	v.cur = ex

	err2 := v.VisitChildren(ctx)
	v.cur = prev
	if err2 != nil {
		return err2.(error)
	}

	return v.cur.AddChild(ex)
}

// VisitCmpr implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitCmpr(ctx *slq.CmprContext) any {
	return v.VisitChildren(ctx)
}

// VisitStmtList implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitStmtList(ctx *slq.StmtListContext) any {
	return nil // not using StmtList just yet
}

// VisitLiteral implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitLiteral(ctx *slq.LiteralContext) any {
	v.log.Debugf("visiting literal: %q", ctx.GetText())

	lit := &Literal{}
	lit.ctx = ctx
	_ = lit.SetParent(v.cur)
	err := v.cur.AddChild(lit)
	return err
}

// VisitUnaryOperator implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitUnaryOperator(ctx *slq.UnaryOperatorContext) any {
	return nil
}

// VisitFnName implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitFnName(ctx *slq.FnNameContext) any {
	return nil
}

// VisitGroup implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitGroup(ctx *slq.GroupContext) any {
	// parent node must be a segment
	_, ok := v.cur.(*Segment)
	if !ok {
		return errorf("parent of GROUP() must be Segment, but got: %T", v.cur)
	}

	sels := ctx.AllSEL()
	if len(sels) == 0 {
		return errorf("GROUP() requires at least one column selector argument")
	}

	grp := &Group{}
	grp.ctx = ctx
	err := v.cur.AddChild(grp)
	if err != nil {
		return err
	}

	for _, selCtx := range sels {
		colSel, err := newColSelector(grp, selCtx, "") // FIXME: Handle alias appropriately
		if err != nil {
			return err
		}
		err = grp.AddChild(colSel)
		if err != nil {
			return err
		}
	}

	return nil
}

// VisitJoin implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitJoin(ctx *slq.JoinContext) any {
	// parent node must be a segment
	seg, ok := v.cur.(*Segment)
	if !ok {
		return errorf("parent of JOIN() must be Segment, but got: %T", v.cur)
	}

	join := &Join{seg: seg, ctx: ctx}
	err := seg.AddChild(join)
	if err != nil {
		return err
	}

	expr := ctx.JoinConstraint()
	if expr == nil {
		return nil
	}

	// the join contains a constraint, let's hit it
	v.cur = join
	err2 := v.VisitJoinConstraint(expr.(*slq.JoinConstraintContext))
	if err2 != nil {
		return err2
	}
	// set cur back to previous
	v.cur = seg
	return nil
}

// VisitJoinConstraint implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitJoinConstraint(ctx *slq.JoinConstraintContext) any {
	joinNode, ok := v.cur.(*Join)
	if !ok {
		return errorf("JOIN constraint must have JOIN parent, but got %T", v.cur)
	}

	// the constraint could be empty
	children := ctx.GetChildren()
	if len(children) == 0 {
		return nil
	}

	// the constraint could be a single SEL (in which case, there's no comparison operator)
	if ctx.Cmpr() == nil {
		// there should be exactly one SEL
		sels := ctx.AllSEL()
		if len(sels) != 1 {
			return errorf("JOIN constraint without a comparison operator must have exactly one selector")
		}

		joinExprNode := &JoinConstraint{join: joinNode, ctx: ctx}

		colSelNode := &Selector{}
		colSelNode.ctx = sels[0]
		colSelNode.parent = joinExprNode

		err := joinExprNode.AddChild(colSelNode)
		if err != nil {
			return err
		}
		return joinNode.AddChild(joinExprNode)
	}

	// we've got a comparison operator
	sels := ctx.AllSEL()
	if len(sels) != 2 {
		// REVISIT: probably unnecessary, should be caught by the parser
		return errorf("JOIN constraint must have 2 operands (left & right), but got %d", len(sels))
	}

	join, ok := v.cur.(*Join)
	if !ok {
		return errorf("JOIN constraint must have JOIN parent, but got %T", v.cur)
	}
	joinCondition := &JoinConstraint{join: join, ctx: ctx}

	leftSel := &Selector{}
	leftSel.ctx = sels[0]
	leftSel.parent = joinCondition
	err := joinCondition.AddChild(leftSel)
	if err != nil {
		return err
	}

	cmpr := newCmpr(joinCondition, ctx.Cmpr())
	err = joinCondition.AddChild(cmpr)
	if err != nil {
		return err
	}

	rightSel := &Selector{}
	rightSel.ctx = sels[1]
	rightSel.parent = joinCondition
	err = joinCondition.AddChild(rightSel)
	if err != nil {
		return err
	}

	return join.AddChild(joinCondition)
}

// VisitTerminal implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitTerminal(ctx antlr.TerminalNode) any {
	v.log.Debugf("visiting terminal: %q", ctx.GetText())

	val := ctx.GetText()

	if isOperator(val) {
		op := &Operator{}
		op.ctx = ctx

		err := op.SetParent(v.cur)
		if err != nil {
			return err
		}

		err = v.cur.AddChild(op)
		if err != nil {
			return err
		}
		return nil
	}

	// Unknown terminal, but that's not a problem.
	return nil
}

// VisitRowRange implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitRowRange(ctx *slq.RowRangeContext) any {
	// []      select all rows (no range)
	// [1]     select row[1]
	// [10:15] select rows 10 thru 15
	// [0:15]  select rows 0 thru 15
	// [:15]   same as above (0 thru 15)
	// [10:]   select all rows from 10 onwards

	if ctx.COLON() == nil && len(ctx.AllNN()) == 0 {
		// [] select all rows, aka no range
		return nil
	}

	if ctx.COLON() == nil {
		// [1] -- select row[1]
		if len(ctx.AllNN()) != 1 {
			return errorf("row range: expected one integer but got %d", len(ctx.AllNN()))
		}

		i, _ := strconv.Atoi(ctx.AllNN()[0].GetText())
		rr := newRowRange(ctx, i, 1)
		return v.cur.AddChild(rr)
	}

	// there's a colon... can only be one or two ints
	if len(ctx.AllNN()) > 2 {
		return errorf("row range: expected one or two integers but got %d", len(ctx.AllNN()))
	}

	if len(ctx.AllNN()) == 2 {
		// [10:15] -- select rows 10 thru 15
		offset, _ := strconv.Atoi(ctx.AllNN()[0].GetText())
		finish, _ := strconv.Atoi(ctx.AllNN()[1].GetText())
		limit := finish - offset
		rr := newRowRange(ctx, offset, limit)
		return v.cur.AddChild(rr)
	}

	// it's one of these two cases:
	//   [:15]   (0 thru 15)
	//   [10:]   select all rows from 10 onwards
	// so we need to determine if the INT is before or after the colon
	var offset int
	limit := -1

	if ctx.COLON().GetSymbol().GetTokenIndex() < ctx.AllNN()[0].GetSymbol().GetTokenIndex() {
		// [:15]   (0 thru 15)
		offset = 0
		limit, _ = strconv.Atoi(ctx.AllNN()[0].GetText())
	} else {
		// [10:]   select all rows from 10 onwards
		offset, _ = strconv.Atoi(ctx.AllNN()[0].GetText())
	}

	rr := newRowRange(ctx, offset, limit)
	return v.cur.AddChild(rr)
}

// VisitErrorNode implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitErrorNode(ctx antlr.ErrorNode) any {
	v.log.Debugf("error node: %v", ctx.GetText())
	return nil
}
