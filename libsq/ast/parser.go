package ast

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/antlr/antlr4/runtime/Go/antlr"
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

	str := append(el.errs, el.warnings...)
	return strings.Join(str, "\n")
}

func (el *antlrErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	text := fmt.Sprintf("%s: syntax error: [%d:%d] %s", el.name, line, column, msg)
	el.errs = append(el.errs, text)
}

func (el *antlrErrorListener) ReportAmbiguity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, exact bool, ambigAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	tok := recognizer.GetCurrentToken()
	text := fmt.Sprintf("%s: syntax ambiguity: [%d:%d]", el.name, startIndex, stopIndex)
	text = text + "  >>" + tok.GetText() + "<<"
	el.warnings = append(el.warnings, text)
}

func (el *antlrErrorListener) ReportAttemptingFullContext(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, conflictingAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	text := fmt.Sprintf("%s: attempting full context: [%d:%d]", el.name, startIndex, stopIndex)
	el.warnings = append(el.warnings, text)
}

func (el *antlrErrorListener) ReportContextSensitivity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex, prediction int, configs antlr.ATNConfigSet) {
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

func (v *parseTreeVisitor) Visit(ctx antlr.ParseTree) interface{} {
	v.log.Debugf("visiting %T: %v: ", ctx, ctx.GetText())

	switch ctx := ctx.(type) {
	case *slq.SegmentContext:
		return v.VisitSegment(ctx)
	case *slq.ElementContext:
		return v.VisitElement(ctx)
	case *slq.DsElementContext:
		return v.VisitDsElement(ctx)
	case *slq.DsTblElementContext:
		return v.VisitDsTblElement(ctx)
	case *slq.SelElementContext:
		return v.VisitSelElement(ctx)
	case *slq.FnContext:
		return v.VisitFn(ctx)
	case *slq.FnNameContext:
		return v.VisitFnName(ctx)
	case *slq.JoinContext:
		return v.VisitJoin(ctx)
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

func (v *parseTreeVisitor) VisitChildren(ctx antlr.RuleNode) interface{} {
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

func (v *parseTreeVisitor) VisitQuery(ctx *slq.QueryContext) interface{} {
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

func (v *parseTreeVisitor) VisitDsElement(ctx *slq.DsElementContext) interface{} {
	ds := &Datasource{}
	ds.parent = v.cur
	ds.ctx = ctx.DATASOURCE()
	return v.cur.AddChild(ds)
}

func (v *parseTreeVisitor) VisitDsTblElement(ctx *slq.DsTblElementContext) interface{} {
	tblSel := &TblSelector{}
	tblSel.parent = v.cur
	tblSel.ctx = ctx

	tblSel.DSName = ctx.DATASOURCE().GetText()
	tblSel.TblName = ctx.SEL().GetText()[1:]

	return v.cur.AddChild(tblSel)
}

func (v *parseTreeVisitor) VisitSegment(ctx *slq.SegmentContext) interface{} {
	seg := &Segment{}
	seg.bn.ctx = ctx
	seg.bn.parent = v.AST

	if v == nil {
		panic("v is nil")
	}

	if v.AST == nil {
		panic("v.AST is nil")
	}

	v.AST.AddSegment(seg)
	v.cur = seg

	return v.VisitChildren(ctx)
}

func (v *parseTreeVisitor) VisitSelElement(ctx *slq.SelElementContext) interface{} {
	selector := &Selector{}
	selector.parent = v.cur
	selector.ctx = ctx.SEL()
	return v.cur.AddChild(selector)
}

func (v *parseTreeVisitor) VisitElement(ctx *slq.ElementContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *parseTreeVisitor) VisitFn(ctx *slq.FnContext) interface{} {
	v.log.Debugf("visiting function: %v", ctx.GetText())

	fn := &Func{fnName: ctx.FnName().GetText()}
	fn.ctx = ctx
	err := fn.SetParent(v.cur)
	if err != nil {
		return err
	}

	prev := v.cur
	v.cur = fn
	err2 := v.VisitChildren(ctx)
	v.cur = prev
	if err2 != nil {
		return err2.(error)
	}

	return v.cur.AddChild(fn)
}

func (v *parseTreeVisitor) VisitExpr(ctx *slq.ExprContext) interface{} {
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

func (v *parseTreeVisitor) VisitCmpr(ctx *slq.CmprContext) interface{} {
	return v.VisitChildren(ctx)
}

func (v *parseTreeVisitor) VisitStmtList(ctx *slq.StmtListContext) interface{} {
	return nil // not using StmtList just yet
}

func (v *parseTreeVisitor) VisitLiteral(ctx *slq.LiteralContext) interface{} {
	v.log.Debugf("visiting literal: %q", ctx.GetText())

	lit := &Literal{}
	lit.ctx = ctx
	_ = lit.SetParent(v.cur)
	err := v.cur.AddChild(lit)
	return err
}

func (v *parseTreeVisitor) VisitUnaryOperator(ctx *slq.UnaryOperatorContext) interface{} {
	return nil
}
func (v *parseTreeVisitor) VisitFnName(ctx *slq.FnNameContext) interface{} {
	return nil
}

func (v *parseTreeVisitor) VisitGroup(ctx *slq.GroupContext) interface{} {
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
		err = grp.AddChild(newColSelector(grp, selCtx))
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *parseTreeVisitor) VisitJoin(ctx *slq.JoinContext) interface{} {
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

func (v *parseTreeVisitor) VisitJoinConstraint(ctx *slq.JoinConstraintContext) interface{} {
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

	cmpr := newCmnr(joinCondition, ctx.Cmpr())
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

func (v *parseTreeVisitor) VisitTerminal(ctx antlr.TerminalNode) interface{} {
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

	v.log.Warnf("unknown terminal: %q", val)

	return nil
}

func (v *parseTreeVisitor) VisitRowRange(ctx *slq.RowRangeContext) interface{} {
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
	var limit = -1

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

func (v *parseTreeVisitor) VisitErrorNode(ctx antlr.ErrorNode) interface{} {
	v.log.Debugf("error node: %v", ctx.GetText())
	return nil
}
