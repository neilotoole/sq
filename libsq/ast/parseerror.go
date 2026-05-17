// Package ast — see ast.go for package docs.
package ast

import (
	"fmt"
	"strings"
)

// ParseError is the structured error returned by Parse when SLQ input
// fails to lex or parse. It carries enough information for callers to
// render a position-highlighted, human-friendly message.
type ParseError struct {
	// Input is the original SLQ query text that was being parsed.
	Input string

	// Issues is one entry per SyntaxError fired by ANTLR. ANTLR's
	// error-recovery strategy can fire multiple errors per query;
	// Issues preserves the order they were reported.
	Issues []ParseIssue
}

// ParseIssue describes a single syntax error.
type ParseIssue struct {
	// Stage is "lexer" or "parser".
	Stage string

	// Token is the text of the offending token, or "" for lexer errors.
	Token string

	// Msg is the sq-flavored human-friendly message.
	Msg string

	// Suggestion is an optional did-you-mean candidate.
	Suggestion string

	// Line is the 1-based line number where the issue was detected.
	Line int

	// Col is the 0-based column on Line where the issue was detected.
	Col int

	// StartChar is the 0-based char offset into ParseError.Input where
	// the offending span begins. -1 if not available.
	StartChar int

	// StopChar is the 0-based, inclusive char offset where the offending
	// span ends. -1 if not available.
	StopChar int
}

// Error implements error. Returns a single-line summary suitable for
// logs. Rich rendering lives in cli/output/commonw.
func (e *ParseError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "parser: syntax error"
	}
	parts := make([]string, 0, len(e.Issues))
	for _, iss := range e.Issues {
		parts = append(parts, fmt.Sprintf("syntax error at line %d, col %d: %s",
			iss.Line, iss.Col, iss.Msg))
	}
	return strings.Join(parts, "; ")
}

// First returns the first issue, or nil if Issues is empty.
func (e *ParseError) First() *ParseIssue {
	if e == nil || len(e.Issues) == 0 {
		return nil
	}
	return &e.Issues[0]
}
