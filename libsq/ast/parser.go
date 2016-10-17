package ast

import (
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/slq"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

// Parser processes SLQ input text according to the rules of the SLQ grammar.
type Parser struct {
}

// NewParser returns a new Parser.
func NewParser() *Parser {
	return &Parser{}
}

// Parse processes SLQ input text and returns a parse tree. It executes both
// lexer and parser phases.
func (pr *Parser) Parse(slqInput string) (*slq.QueryContext, error) {
	is := antlr.NewInputStream(slqInput)

	lexErrs := &antlrErrorListener{name: "lexer"}
	parseErrs := &antlrErrorListener{name: "parser"}

	lex := slq.NewSLQLexer(is)
	lex.RemoveErrorListeners() // the generated lexer has default listeners we don't want
	lex.AddErrorListener(lexErrs)

	ts := antlr.NewCommonTokenStream(lex, 0)
	p := slq.NewSLQParser(ts)
	p.RemoveErrorListeners() // the generated parser has default listeners we don't want
	p.AddErrorListener(parseErrs)

	qctx := p.Query()
	err := lexErrs.error()
	if err != nil {
		return nil, err
	}
	err = parseErrs.error()
	if err != nil {
		return nil, err
	}

	return qctx.(*slq.QueryContext), nil
}

type antlrErrorListener struct {
	name     string
	errs     []string
	warnings []string
	err      error
}

func (el *antlrErrorListener) error() error {

	if el.err == nil && len(el.errs) > 0 {
		msg := strings.Join(el.errs, "\n")
		el.err = &ParseError{msg: msg}
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
	el.errs = append(el.errs, fmt.Sprintf(text))
	//fmt.Fprintln(os.Stderr, "line "+strconv.Itoa(line)+":"+strconv.Itoa(column)+" "+msg)
}

func (el *antlrErrorListener) ReportAmbiguity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, exact bool, ambigAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	text := fmt.Sprintf("%s: syntax ambiguity: [%d:%d]", el.name, startIndex, stopIndex)
	el.errs = append(el.errs, fmt.Sprintf(text))
}

func (el *antlrErrorListener) ReportAttemptingFullContext(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex int, conflictingAlts *antlr.BitSet, configs antlr.ATNConfigSet) {
	text := fmt.Sprintf("%s: attempting full context: [%d:%d]", el.name, startIndex, stopIndex)
	el.warnings = append(el.warnings, fmt.Sprintf(text))
}

func (el *antlrErrorListener) ReportContextSensitivity(recognizer antlr.Parser, dfa *antlr.DFA, startIndex, stopIndex, prediction int, configs antlr.ATNConfigSet) {
	text := fmt.Sprintf("%s: context sensitivity: [%d:%d]", el.name, startIndex, stopIndex)
	el.warnings = append(el.warnings, fmt.Sprintf(text))
}

// ParseError represents an error in lexing/parsing input.
type ParseError struct {
	msg string
	// TODO: parse error should include more detail, such as the offending token, position, etc.
}

func (p *ParseError) Error() string {
	return p.msg
}
