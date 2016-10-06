// Package ql (Query Language) implements sq's Simple Query language.
package ql

import (
	"fmt"

	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/out"
	"github.com/neilotoole/sq/lib/ql/parser"
	"github.com/pboyer/antlr4/runtime/Go/antlr"
)

var sourceSet *driver.SourceSet

// SetSourceSet sets the source set available to the package. This func should
// be called to initialize the package before invoking it.
func SetSourceSet(ss *driver.SourceSet) {
	sourceSet = ss
}

//
//func ToSQL(query string) (string, error) {
//
//	lg.Debugf("sq query: %q", query)
//	is := antlr.NewInputStream(query)
//
//	lexListener := &antlrErrorListener{name: "lexer"}
//	parseListener := &antlrErrorListener{name: "parser"}
//
//	lex := parser.NewSQLexer(is)
//	lex.RemoveErrorListeners()
//	lex.AddErrorListener(lexListener)
//
//	ts := antlr.NewCommonTokenStream(lex, 0)
//	p := parser.NewSQParser(ts)
//	p.RemoveErrorListeners()
//	p.AddErrorListener(parseListener)
//
//	q := p.Query()
//	lg.Debugf("%s", lexListener)
//	lg.Debugf("%s", parseListener)
//
//	err := lexListener.error()
//	if err != nil {
//		return "", err
//	}
//	err = parseListener.error()
//	if err != nil {
//		return "", err
//	}
//
//	tree, err := BuildAST(q)
//	if err != nil {
//		return "", err
//	}
//	//lg.Debugf("AST:\n%s", ast.ToTreeString(tree))
//
//	stmt,  err := BuildModel(tree)
//	if err != nil {
//		return "", err
//	}
//
//	lg.Debugf("got stmt2: %v", stmt2)
//
//	renderer := translator.NewRenderer("mysql")
//	sql, err := renderer.Select(stmt)
//
//	lg.Debugf("SQL query: %q", sql)
//	return sql, err
//
//}
func ExecuteSQ(query string, writer out.ResultWriter) error {

	lg.Debugf("sq query: %q", query)
	is := antlr.NewInputStream(query)

	lexListener := &antlrErrorListener{name: "lexer"}
	parseListener := &antlrErrorListener{name: "parser"}

	lex := parser.NewSQLexer(is)
	lex.RemoveErrorListeners()
	lex.AddErrorListener(lexListener)

	ts := antlr.NewCommonTokenStream(lex, 0)
	p := parser.NewSQParser(ts)
	p.RemoveErrorListeners()
	p.AddErrorListener(parseListener)

	q := p.Query()
	lg.Debugf("%s", lexListener)
	lg.Debugf("%s", parseListener)

	err := lexListener.error()
	if err != nil {
		return err
	}
	err = parseListener.error()
	if err != nil {
		return err
	}

	ast, err := BuildAST(q)
	if err != nil {
		return err
	}

	stmt, err := BuildModel(ast)
	if err != nil {
		return err
	}

	lg.Debugf("got stmt2: %v", stmt)

	xng := NewXEngine(stmt, writer)
	err = xng.Execute()
	return err

	//renderer := translator.NewRenderer("mysql")
	//sql, err := renderer.Select(stmt)
	//
	//lg.Debugf("SQL query: %q", sql)
	//
	//ds, err := sourceSet.Get(ast.Datasource)
	//if err != nil {
	//	return err
	//}
	//
	//database, err := NewDatabase(ds)
	//if err != nil {
	//	return err
	//}
	//
	//err = database.Execute(sql, writer)
	//return err

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
		el.err = NewParseError(msg)
	}

	return el.err
}

func (el *antlrErrorListener) String() string {
	if len(el.errs)+len(el.warnings) == 0 {
		return fmt.Sprintf("%s: no issues", el.name)
	}

	//str := make([]string, 0, len(el.errs) + len(el.warnings))

	str := append(el.errs, el.warnings...)
	return strings.Join(str, "\n")

	//str := make([]string, len(el.errs)+len(el.warnings))
	//for i, msg := range el.errs {
	//	str[i] = msg
	//}
	//
	//for i, msg := range el.warnings {
	//	str[i] = msg
	//}
	//
	//return strings.Join(str, "\n")
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

type ParseError struct {
	msg string
}

func (p *ParseError) Error() string {
	return p.msg
}

func NewParseError(msg string) error {
	return &ParseError{msg: msg}
}
