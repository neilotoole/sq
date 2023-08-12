package ast

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/antlr4-go/antlr/v4"
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
	if err := lexErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	if err := parseErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	return qCtx.(*slq.QueryContext), nil
}

var _ antlr.ErrorListener = (*antlrErrorListener)(nil)

type antlrErrorListener struct {
	log      *slog.Logger
	name     string
	errs     []string
	warnings []string
	err      error
}

// SyntaxError implements antlr.ErrorListener.
//
//nolint:revive
func (el *antlrErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{},
	line, column int, msg string, e antlr.RecognitionException,
) {
	text := fmt.Sprintf("%s: syntax error: [%d:%d] %s", el.name, line, column, msg)
	el.errs = append(el.errs, text)
}

// ReportAmbiguity implements antlr.ErrorListener.
//
//nolint:revive
func (el *antlrErrorListener) ReportAmbiguity(recognizer antlr.Parser, dfa *antlr.DFA,
	startIndex, stopIndex int, exact bool, ambigAlts *antlr.BitSet, configs *antlr.ATNConfigSet,
) {
	tok := recognizer.GetCurrentToken()
	text := fmt.Sprintf("%s: syntax ambiguity: [%d:%d]", el.name, startIndex, stopIndex)
	text = text + "  >>" + tok.GetText() + "<<"
	el.warnings = append(el.warnings, text)
}

// ReportAttemptingFullContext implements antlr.ErrorListener.
//
//nolint:revive
func (el *antlrErrorListener) ReportAttemptingFullContext(recognizer antlr.Parser, dfa *antlr.DFA,
	startIndex, stopIndex int, conflictingAlts *antlr.BitSet, configs *antlr.ATNConfigSet,
) {
	text := fmt.Sprintf("%s: attempting full context: [%d:%d]", el.name, startIndex, stopIndex)
	el.warnings = append(el.warnings, text)
}

// ReportContextSensitivity implements antlr.ErrorListener.
//
//nolint:revive
func (el *antlrErrorListener) ReportContextSensitivity(recognizer antlr.Parser, dfa *antlr.DFA,
	startIndex, stopIndex, prediction int, configs *antlr.ATNConfigSet,
) {
	text := fmt.Sprintf("%s: context sensitivity: [%d:%d]", el.name, startIndex, stopIndex)
	el.warnings = append(el.warnings, text)
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

// parseError represents an error in lexing/parsing input.
type parseError struct {
	msg string
	// TODO: parse error should include more detail, such as
	// the offending token, position, etc.
}

// Error implements error.
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
	case *slq.JoinTableContext:
		return v.VisitJoinTable(ctx)
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
	case *slq.WhereContext:
		return v.VisitWhere(ctx)
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

// VisitElement implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitElement(ctx *slq.ElementContext) any {
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
		op.text = ctx.GetText()

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

// VisitErrorNode implements slq.SLQVisitor.
func (v *parseTreeVisitor) VisitErrorNode(ctx antlr.ErrorNode) any {
	v.log.Debug("Error node", lga.Val, ctx.GetText())
	return nil
}
