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
	// This value is modified as the tree is descended.
	cur Node

	ast *AST
}

// using is a convenience function that sets v.cur to cur,
// executes fn, and then restores v.cur to its previous value.
// The type of the returned value is declared as "any" instead of
// error, because that's the generated antlr code returns "any".
func (v *parseTreeVisitor) using(node Node, fn func() any) any {
	prev := v.cur
	v.cur = node
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
	case *slq.HandleContext:
		return v.VisitHandle(ctx)
	case *slq.HandleTableContext:
		return v.VisitHandleTable(ctx)
	case *slq.SelectorContext:
		return v.VisitSelector(ctx)
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
	case *slq.OrderByContext:
		return v.VisitOrderBy(ctx)
	case *slq.OrderByTermContext:
		return v.VisitOrderByTerm(ctx)
	case *slq.LiteralContext:
		return v.VisitLiteral(ctx)
	case *antlr.TerminalNodeImpl:
		return v.VisitTerminal(ctx)
	case *slq.SelectorElementContext:
		return v.VisitSelectorElement(ctx)
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
	v.ast = &AST{}
	v.ast.ctx = ctx
	v.ast.text = ctx.GetText()
	v.cur = v.ast

	for _, seg := range ctx.AllSegment() {
		err := v.VisitSegment(seg.(*slq.SegmentContext))
		if err != nil {
			return err
		}
	}

	return nil
}

// VisitHandle implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitHandle(ctx *slq.HandleContext) any {
	ds := &HandleNode{}
	ds.parent = v.cur
	ds.ctx = ctx.HANDLE()
	return v.cur.AddChild(ds)
}

// VisitHandleTable implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitHandleTable(ctx *slq.HandleTableContext) any {
	selNode := &TblSelectorNode{}
	selNode.parent = v.cur
	selNode.ctx = ctx

	selNode.handle = ctx.HANDLE().GetText()

	var err error
	if selNode.tblName, err = extractSelVal(ctx.NAME()); err != nil {
		return err
	}

	return v.cur.AddChild(selNode)
}

// VisitSegment implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSegment(ctx *slq.SegmentContext) any {
	seg := newSegmentNode(v.ast, ctx)
	v.ast.AddSegment(seg)
	v.cur = seg

	return v.VisitChildren(ctx)
}

// VisitOrderBy implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitOrderBy(ctx *slq.OrderByContext) interface{} {
	node := &OrderByNode{}
	node.parent = v.cur
	node.ctx = ctx
	node.text = ctx.GetText()

	if err := v.cur.AddChild(node); err != nil {
		return err
	}

	return v.using(node, func() any {
		// This will result in VisitOrderByTerm being called on the children.
		return v.VisitChildren(ctx)
	})
}

// VisitOrderByTerm implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitOrderByTerm(ctx *slq.OrderByTermContext) interface{} {
	node := &OrderByTermNode{}
	node.parent = v.cur
	node.ctx = ctx
	node.text = ctx.GetText()

	selNode, err := newSelectorNode(node, ctx.Selector())
	if err != nil {
		return nil
	}

	if ctx.ORDER_ASC() != nil {
		node.direction = OrderByDirectionAsc
	} else if ctx.ORDER_DESC() != nil {
		node.direction = OrderByDirectionDesc
	}

	if err := node.AddChild(selNode); err != nil {
		return err
	}

	return v.cur.AddChild(node)
}

// VisitSelectorElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSelectorElement(ctx *slq.SelectorElementContext) any {
	selNode, err := newSelectorNode(v.cur, ctx.Selector())
	if err != nil {
		return err
	}

	if aliasCtx := ctx.Alias(); aliasCtx != nil {
		selNode.alias = ctx.Alias().ID().GetText()
	}

	if err := v.cur.AddChild(selNode); err != nil {
		return err
	}

	// No need to descend to the children, because we've already dealt
	// with them in this function.
	return nil
}

// VisitSelector implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSelector(ctx *slq.SelectorContext) any {
	// no-op
	return nil
}

// VisitElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitElement(ctx *slq.ElementContext) any {
	return v.VisitChildren(ctx)
}

// VisitAlias implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitAlias(ctx *slq.AliasContext) any {
	// TODO: Probably don't need this.
	alias := ctx.ID().GetText()

	switch node := v.cur.(type) {
	case *SelectorNode:
		node.alias = alias
	case *FuncNode:
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

		// VisitAlias will expect v.cur to be a FuncNode.
		lastNode := nodeLastChild(v.cur)
		fnNode, ok := lastNode.(*FuncNode)
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

	fn := &FuncNode{fnName: ctx.FnName().GetText()}
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

	// check if the expr is a selector, e.g. ".uid"
	if selCtx := ctx.Selector(); selCtx != nil {
		selNode, err := newSelectorNode(v.cur, selCtx)
		if err != nil {
			return err
		}
		return v.cur.AddChild(selNode)
	}

	if ctx.Literal() != nil {
		return v.VisitLiteral(ctx.Literal().(*slq.LiteralContext))
	}

	ex := &ExprNode{}
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

	lit := &LiteralNode{}
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
	_, ok := v.cur.(*SegmentNode)
	if !ok {
		return errorf("parent of GROUP() must be SegmentNode, but got: %T", v.cur)
	}

	// FIXME: this needs to be revisited.
	sels := ctx.AllSelector()
	if len(sels) == 0 {
		return errorf("GROUP() requires at least one column selector argument")
	}

	grp := &GroupByNode{}
	grp.ctx = ctx
	err := v.cur.AddChild(grp)
	if err != nil {
		return err
	}

	for _, selCtx := range sels {
		colSel, err := newSelectorNode(grp, selCtx)
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
	seg, ok := v.cur.(*SegmentNode)
	if !ok {
		return errorf("parent of JOIN() must be SegmentNode, but got: %T", v.cur)
	}

	join := &JoinNode{seg: seg, ctx: ctx}
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
	joinNode, ok := v.cur.(*JoinNode)
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
		sels := ctx.AllSelector()
		if len(sels) != 1 {
			return errorf("JOIN constraint without a comparison operator must have exactly one selector")
		}

		joinExprNode := &JoinConstraint{join: joinNode, ctx: ctx}

		colSelNode, err := newSelectorNode(joinExprNode, sels[0])
		if err != nil {
			return err
		}

		if err := joinExprNode.AddChild(colSelNode); err != nil {
			return err
		}

		return joinNode.AddChild(joinExprNode)
	}

	// We've got a comparison operator
	sels := ctx.AllSelector()
	if len(sels) != 2 {
		// REVISIT: probably unnecessary, should be caught by the parser
		return errorf("JOIN constraint must have 2 operands (left & right), but got %d", len(sels))
	}

	join, ok := v.cur.(*JoinNode)
	if !ok {
		return errorf("JoinConstraint must have JoinNode parent, but got %T", v.cur)
	}
	joinCondition := &JoinConstraint{join: join, ctx: ctx}

	leftSel, err := newSelectorNode(joinCondition, sels[0])
	if err != nil {
		return err
	}

	if err = joinCondition.AddChild(leftSel); err != nil {
		return err
	}

	cmpr := newCmpr(joinCondition, ctx.Cmpr())
	if err = joinCondition.AddChild(cmpr); err != nil {
		return err
	}

	rightSel, err := newSelectorNode(joinCondition, sels[1])
	if err != nil {
		return err
	}

	if err = joinCondition.AddChild(rightSel); err != nil {
		return err
	}

	return join.AddChild(joinCondition)
}

// VisitTerminal implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitTerminal(ctx antlr.TerminalNode) any {
	v.log.Debugf("visiting terminal: %q", ctx.GetText())

	val := ctx.GetText()

	if isOperator(val) {
		op := &OperatorNode{}
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
		rr := newRowRangeNode(ctx, i, 1)
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
		rr := newRowRangeNode(ctx, offset, limit)
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

	rr := newRowRangeNode(ctx, offset, limit)
	return v.cur.AddChild(rr)
}

// VisitErrorNode implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitErrorNode(ctx antlr.ErrorNode) any {
	v.log.Debugf("error node: %v", ctx.GetText())
	return nil
}
