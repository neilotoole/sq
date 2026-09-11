package ast

import (
	"fmt"
	"log/slog"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/libsq/ast/internal/slq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// parseSLQ processes SLQ input text and returns a parse tree. It
// executes both lexer and parser phases.
func parseSLQ(log *slog.Logger, input string) (*slq.QueryContext, error) {
	lex := slq.NewSLQLexer(antlr.NewInputStream(input))
	lex.RemoveErrorListeners() // the generated lexer has default listeners we don't want
	lexErrs := &antlrErrorListener{name: "lexer", log: log, input: input}
	lex.AddErrorListener(lexErrs)

	p := slq.NewSLQParser(antlr.NewCommonTokenStream(lex, 0))
	p.RemoveErrorListeners() // the generated parser has default listeners we don't want
	parseErrs := &antlrErrorListener{name: "parser", log: log, input: input}
	p.AddErrorListener(parseErrs)

	qCtx := p.Query()
	lexErrs.logDiagnostics()
	parseErrs.logDiagnostics()
	if err := lexErrs.error(); err != nil {
		return nil, errz.Err(err)
	}
	if err := parseErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	return qCtx.(*slq.QueryContext), nil
}

var _ antlr.ErrorListener = (*antlrErrorListener)(nil)

// antlrErrorListener collects lexer and parser errors emitted by ANTLR
// during parseSLQ and converts them into a structured ParseError. One
// instance is attached to the lexer and a separate instance to the
// parser; their issues are checked independently by parseSLQ.
type antlrErrorListener struct {
	// recognizer is the parser/lexer the listener is attached to. Used
	// to look up literal names when building did-you-mean suggestions.
	recognizer antlr.Recognizer
	log        *slog.Logger
	name       string // "lexer" or "parser"
	input      string
	issues     []ParseIssue
	warnings   []string
	// inputRunes lazily caches []rune(input) so multiple lexer errors on the
	// same input don't each re-convert the whole string. See inputRuneSlice.
	inputRunes []rune
}

// inputRuneSlice returns input as a rune slice, converting once and caching
// the result for reuse across multiple lexer errors.
func (el *antlrErrorListener) inputRuneSlice() []rune {
	if el.inputRunes == nil {
		el.inputRunes = []rune(el.input)
	}
	return el.inputRunes
}

// SyntaxError implements antlr.ErrorListener.
func (el *antlrErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol any,
	line, column int, msg string, _ antlr.RecognitionException,
) {
	iss := ParseIssue{
		stage: el.name,
		Line:  line,
		Col:   column,
		Msg:   "",
	}

	if tok, ok := offendingSymbol.(antlr.Token); ok && tok != nil {
		iss.Token = tok.GetText()
		// The synthetic <EOF> token reports Stop == Start-1, yielding an
		// empty span (see Span.Empty) that marks the end-of-input position.
		iss.Span = &Span{Start: tok.GetStart(), Stop: tok.GetStop()}
	}

	// Lexer-stage errors don't carry a token. ANTLR reports the position as
	// (line, column) where column is 0-based *within the line*, not an
	// absolute offset into the input. Convert to an absolute rune offset so
	// multi-line input maps to the correct rune, then synthesize a
	// single-character span there (matching the terse message produced below).
	if iss.Token == "" && el.name == "lexer" {
		runes := el.inputRuneSlice()
		if off := runeOffsetForLineCol(runes, line, column); off >= 0 && off < len(runes) {
			iss.Token = string(runes[off])
			iss.Span = &Span{Start: off, Stop: off}
		}
	}

	iss.Msg = buildIssueMsg(iss.Token, msg)

	// Capture the recognizer on first call; used below to resolve expected
	// token IDs to literal names when computing did-you-mean suggestions.
	if el.recognizer == nil {
		el.recognizer = recognizer
	}

	// Compute did-you-mean Suggestion (parser stage only; lexer doesn't
	// expose an expected-token set). The expected-types slice is intentionally
	// local — it's only used here.
	if iss.Token != "" {
		if p, ok := recognizer.(antlr.Parser); ok {
			if set := p.GetExpectedTokens(); set != nil {
				ivs := set.GetIntervals()
				pairs := make([][2]int, 0, len(ivs))
				for _, iv := range ivs {
					pairs = append(pairs, [2]int{iv.Start, iv.Stop})
				}
				expectedTypes := collectExpectedTokenTypes(pairs)
				if len(expectedTypes) > 0 {
					literals := el.recognizer.GetLiteralNames()
					candidates := expectedTokenLiterals(expectedTypes, literals)
					iss.Suggestion = suggestForToken(iss.Token, candidates)
				}
			}
		}
	}

	el.issues = append(el.issues, iss)
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
	if len(el.issues) == 0 {
		return nil
	}
	return &ParseError{Input: el.input, Issues: el.issues}
}

// logDiagnostics emits the listener's collected issues and warnings to its
// logger at debug level. No-op when there's no logger or nothing to report.
func (el *antlrErrorListener) logDiagnostics() {
	if el.log == nil || len(el.issues)+len(el.warnings) == 0 {
		return
	}
	el.log.Debug("SLQ parse diagnostics", lga.Val, el.String())
}

func (el *antlrErrorListener) String() string {
	if len(el.issues)+len(el.warnings) == 0 {
		return el.name + ": no issues"
	}
	strs := make([]string, 0, len(el.issues)+len(el.warnings))
	for _, iss := range el.issues {
		strs = append(strs, fmt.Sprintf("%s: syntax error: [%d:%d] %s",
			iss.stage, iss.Line, iss.Col, iss.Msg))
	}
	strs = append(strs, el.warnings...)
	return strings.Join(strs, "\n")
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
	v.log.Debug(
		"Visit",
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
	case *slq.HavingContext:
		return v.VisitHaving(ctx)
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

// runeOffsetForLineCol converts a 1-based line and 0-based column (as
// reported by ANTLR's SyntaxError, where column is relative to the start of
// the line) into an absolute 0-based rune offset into runes. Lines are
// delimited by '\n', matching the convention ANTLR uses to advance its line
// counter. It returns -1 if line < 1, col < 0, or line is beyond the input.
// A col that overshoots the line is not detected — the returned offset may
// be >= len(runes), so callers must bounds-check.
func runeOffsetForLineCol(runes []rune, line, col int) int {
	if line < 1 || col < 0 {
		return -1
	}
	i, cur := 0, 1
	for i < len(runes) && cur < line {
		if runes[i] == '\n' {
			cur++
		}
		i++
	}
	if cur < line {
		return -1 // line beyond input
	}
	return i + col
}

// buildIssueMsg produces a terse, sq-flavored error message. When token
// is non-empty it returns "unexpected '<token>'" (or "unexpected end of
// input" for the synthetic <EOF> token), ignoring antlrMsg. For lexer
// errors with no token, it falls back to antlrMsg with ANTLR's
// "token recognition error at: " prefix stripped.
func buildIssueMsg(token, antlrMsg string) string {
	if token != "" && token != "<EOF>" {
		return fmt.Sprintf("unexpected '%s'", token)
	}
	if token == "<EOF>" {
		return "unexpected end of input"
	}
	// Lexer error: no token formed. ANTLR's lexer messages are
	// usually compact and useful as-is (e.g. "token recognition
	// error at: '#'"). Strip the "token recognition error at: "
	// prefix to make it terser.
	const lexerPrefix = "token recognition error at: "
	if after, ok := strings.CutPrefix(antlrMsg, lexerPrefix); ok {
		return "unexpected " + after
	}
	return antlrMsg
}
