package ast

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/stringz"

	"golang.org/x/exp/slog"

	"github.com/antlr/antlr4/runtime/Go/antlr/v4"
	"github.com/neilotoole/sq/libsq/ast/internal/slq"
)

// parseSLQ processes SLQ input text according to the rules of the SQL grammar,
// and returns a parse tree. It executes both lexer and parser phases.
func parseSLQ(log *slog.Logger, input string) (*slq.QueryContext, error) {
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
	log      *slog.Logger
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
//
//nolint:revive
func (el *antlrErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol any, line, column int,
	msg string, e antlr.RecognitionException,
) {
	text := fmt.Sprintf("%s: syntax error: [%d:%d] %s", el.name, line, column, msg)
	el.errs = append(el.errs, text)
}

// ReportAmbiguity implements antlr.ErrorListener.
//
//nolint:revive
func (el *antlrErrorListener) ReportAmbiguity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int,
	exact bool, ambigAlts *antlr.BitSet, configs antlr.ATNConfigSet,
) {
	tok := recognizer.GetCurrentToken()
	text := fmt.Sprintf("%s: syntax ambiguity: [%d:%d]", el.name, startIndex, stopIndex)
	text = text + "  >>" + tok.GetText() + "<<"
	el.warnings = append(el.warnings, text)
}

// ReportAttemptingFullContext implements antlr.ErrorListener.
//
//nolint:revive
func (el *antlrErrorListener) ReportAttemptingFullContext(recognizer antlr.Parser, dfa *antlr.DFA, startIndex,
	stopIndex int, conflictingAlts *antlr.BitSet, configs antlr.ATNConfigSet,
) {
	text := fmt.Sprintf("%s: attempting full context: [%d:%d]", el.name, startIndex, stopIndex)
	el.warnings = append(el.warnings, text)
}

// ReportContextSensitivity implements antlr.ErrorListener.
//
//nolint:revive
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
	log *slog.Logger

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
	v.log.Debug("Visit",
		lga.Type, stringz.Type(ctx),
		lga.Text, ctx.GetText(),
	)

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
	case *slq.FuncElementContext:
		return v.VisitFuncElement(ctx)
	case *slq.FuncContext:
		return v.VisitFunc(ctx)
	case *slq.FuncNameContext:
		return v.VisitFuncName(ctx)
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
	case *slq.ExprElementContext:
		return v.VisitExprElement(ctx)
	case *slq.ExprContext:
		return v.VisitExpr(ctx)
	case *slq.GroupByContext:
		return v.VisitGroupBy(ctx)
	case *slq.GroupByTermContext:
		return v.VisitGroupByTerm(ctx)
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
	case *slq.UniqueFuncContext:
		return v.VisitUniqueFunc(ctx)
	case *slq.CountFuncContext:
		return v.VisitCountFunc(ctx)
	case *slq.ArgContext:
		return v.VisitArg(ctx)
	}

	// should never be reached
	return errorf("unknown node type: %T", ctx)
}

// VisitChildren implements antlr.ParseTreeVisitor.
func (v *parseTreeVisitor) VisitChildren(ctx antlr.RuleNode) any {
	for _, child := range ctx.GetChildren() {
		tree, ok := child.(antlr.ParseTree)
		if !ok {
			return errorf("unknown child node type: %T(%s)", child, child.GetPayload())
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

// VisitSelectorElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSelectorElement(ctx *slq.SelectorElementContext) any {
	node, err := newSelectorNode(v.cur, ctx.Selector())
	if err != nil {
		return err
	}

	if aliasCtx := ctx.Alias(); aliasCtx != nil {
		node.alias = ctx.Alias().ID().GetText()
	}

	if err := v.cur.AddChild(node); err != nil {
		return err
	}

	// No need to descend to the children, because we've already dealt
	// with them in this function.
	return nil
}

// VisitSelector implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitSelector(ctx *slq.SelectorContext) any {
	node, err := newSelectorNode(v.cur, ctx)
	if err != nil {
		return err
	}

	return v.cur.AddChild(node)
}

// VisitElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitElement(ctx *slq.ElementContext) any {
	return v.VisitChildren(ctx)
}

// VisitAlias implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitAlias(ctx *slq.AliasContext) any {
	if ctx.ID() == nil && ctx.GetText() == "" {
		return nil
	}

	var alias string
	if ctx.ID() != nil {
		alias = ctx.ID().GetText()
	}

	switch node := v.cur.(type) {
	case *SelectorNode:
		node.alias = alias
	case *FuncNode:
		if alias != "" {
			node.alias = alias
			return nil
		}

		// HACK: The grammar has a dodgy hack to deal with no-arg funcs
		// with an alias that is a reserved word.
		//
		// For example, let's start with this snippet. Note that "count" is
		// a function, equivalent to count().
		//
		//   .actor | count
		//
		// Then add an alias that is a reserved word, such as a function name.
		// In this example, we will use an alias of "count" as well.
		//
		//   .actor | count:count
		//
		// Well, the grammar doesn't know how to handle this. Most likely the
		// grammar could be refactored to deal with this more gracefully. The
		// hack is to look at the full text of the context (e.g. ":count"),
		// instead of just ID, and look for the alias after the colon.

		text := ctx.GetText()
		node.alias = strings.TrimPrefix(text, ":")

	default:
		return errorf("alias not allowed for type %T: %v", node, ctx.GetText())
	}

	return nil
}

// VisitCmpr implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitCmpr(ctx *slq.CmprContext) any {
	return v.VisitChildren(ctx)
}

// VisitStmtList implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitStmtList(_ *slq.StmtListContext) any {
	return nil // not using StmtList just yet
}

// VisitUnaryOperator implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitUnaryOperator(_ *slq.UnaryOperatorContext) any {
	return nil
}

// VisitTerminal implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitTerminal(ctx antlr.TerminalNode) any {
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
	v.log.Debug("error node: %v", ctx.GetText())
	return nil
}
