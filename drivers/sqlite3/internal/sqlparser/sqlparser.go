// Package sqlparser contains SQL parsing functionality for SQLite.
package sqlparser

//go:generate ./generate.sh

import (
	"fmt"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"

	"github.com/neilotoole/sq/drivers/sqlite3/internal/sqlparser/sqlite"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// ExtractTableIdentFromCreateTableStmt extracts table name (and the
// table's schema if specified) from a CREATE TABLE statement.
// If err is nil, table is guaranteed to be non-empty. If arg unescape is
// true, any surrounding quotation chars are trimmed from the returned values.
//
//	CREATE TABLE "sakila"."actor" ( actor_id INTEGER NOT NULL)  -->  sakila, actor, nil
func ExtractTableIdentFromCreateTableStmt(stmt string, unescape bool) (schema, table string, err error) {
	stmtCtx, err := parseCreateTableStmt(stmt)
	if err != nil {
		return "", "", err
	}

	if n, ok := stmtCtx.Schema_name().(*sqlite.Schema_nameContext); ok {
		if n.Any_name() != nil && !n.Any_name().IsEmpty() && n.Any_name().IDENTIFIER() != nil {
			schema = n.Any_name().IDENTIFIER().GetText()
			if unescape {
				schema = trimIdentQuotes(schema)
			}
		}
	}

	if x, ok := stmtCtx.Table_name().(*sqlite.Table_nameContext); ok {
		if x.Any_name() != nil && !x.Any_name().IsEmpty() && x.Any_name().IDENTIFIER() != nil {
			table = x.Any_name().IDENTIFIER().GetText()
			if unescape {
				table = trimIdentQuotes(table)
			}
		}
	}

	if table == "" {
		return "", "", errz.Errorf("failed to extract table name from CREATE TABLE statement")
	}

	return schema, table, nil
}

// trimIdentQuotes trims any of the legal quote characters from s.
// These are double quote, single quote, backtick, and square brackets.
//
//	[actor] -> actor
//	"actor" -> actor
//	`actor` -> actor
//	'actor' -> actor
//
// If s is empty, unquoted, or is malformed, it is returned unchanged.
func trimIdentQuotes(s string) string {
	if len(s) < 2 {
		return s
	}

	switch s[0] {
	case '"', '`', '\'':
		if s[len(s)-1] == s[0] {
			return s[1 : len(s)-1]
		}
	case '[':
		if s[len(s)-1] == ']' {
			return s[1 : len(s)-1]
		}
	default:
	}
	return s
}

func parseCreateTableStmt(input string) (*sqlite.Create_table_stmtContext, error) {
	lex := sqlite.NewSQLiteLexer(antlr.NewInputStream(input))
	lex.RemoveErrorListeners() // the generated lexer has default listeners we don't want
	lexErrs := &antlrErrorListener{name: "lexer"}
	lex.AddErrorListener(lexErrs)

	p := sqlite.NewSQLiteParser(antlr.NewCommonTokenStream(lex, 0))
	p.RemoveErrorListeners() // the generated parser has default listeners we don't want
	parseErrs := &antlrErrorListener{name: "parser"}
	p.AddErrorListener(parseErrs)

	qCtx := p.Create_table_stmt()

	if err := lexErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	if err := parseErrs.error(); err != nil {
		return nil, errz.Err(err)
	}

	return qCtx.(*sqlite.Create_table_stmtContext), nil
}

var _ antlr.ErrorListener = (*antlrErrorListener)(nil)

// antlrErrorListener implements antlr.ErrorListener.
// TODO: this is a copy of the same-named type in libsq/ast/parser.go.
// It should be moved to a common package.
type antlrErrorListener struct {
	err      error
	name     string
	errs     []string
	warnings []string
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
		return el.name + ": no issues"
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
