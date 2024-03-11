// Package sqlparser contains SQL parsing functionality for SQLite.
package sqlparser

//go:generate ./generate.sh

import (
	"fmt"
	"strings"

	antlr "github.com/antlr4-go/antlr/v4"
)

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
