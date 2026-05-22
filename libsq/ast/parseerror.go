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

// Span is a rune-offset range identifying offending text within
// ParseError.Input. Both bounds are 0-based rune (Unicode code point)
// offsets — not byte offsets — so they index correctly into []rune(Input)
// even for non-ASCII text. Stop is inclusive.
//
// Normally Start <= Stop. The sole exception is an empty span (see Empty),
// where Stop == Start-1: the synthetic <EOF> token uses this to mark a
// position with no extent (e.g. "unexpected end of input"). Renderers place
// a caret at Start; the JSON wire form omits the offsets for an empty span.
type Span struct {
	// Start is the rune offset where the span begins.
	Start int

	// Stop is the inclusive rune offset where the span ends. For a
	// single-rune span, Stop == Start.
	Stop int
}

// Empty reports whether the span covers no runes (Stop < Start), as for the
// synthetic <EOF> token. An empty span marks the position Start without
// extent.
func (s Span) Empty() bool {
	return s.Stop < s.Start
}

// ParseIssue describes a single syntax error. Field order is dictated by
// govet's fieldalignment (pointer-bearing fields first); it is not otherwise
// significant — the struct is built and read by field name.
type ParseIssue struct {
	// Span is the rune-offset range of the offending text within
	// ParseError.Input, or nil if positional offsets aren't available
	// (some lexer errors). When nil, renderers fall back to Line/Col.
	Span *Span

	// stage is "lexer" or "parser". Diagnostic-only; not surfaced in
	// user-facing output or the JSON wire form. Read only by
	// antlrErrorListener.String(), which parseSLQ logs at debug level.
	stage string

	// Token is the text of the offending token, or "" for lexer errors.
	Token string

	// Msg is the sq-flavored human-friendly message.
	Msg string

	// Suggestion is an optional did-you-mean candidate.
	Suggestion string

	// Line is the 1-based line number where the issue was detected.
	Line int

	// Col is the 0-based column on Line where the issue was detected. It is
	// kept 0-based to match Span's rune offsets and ANTLR's reported column;
	// all output renderings (text and --json alike) use DisplayCol for the
	// 1-based value users and consumers see.
	Col int
}

// DisplayCol returns the 1-based column for output (Col is stored 0-based).
//
// Every output channel reports 1-based line/col: the text error message and
// the --json wire form both route the column through DisplayCol. We emit
// 1-based, not the raw 0-based Col, because that's the universal human
// convention — every compiler (gcc, go, rustc), linter, and editor status bar
// counts line/col from 1. A reader who sees "col 0" for the first column, or
// "col 4" for the 5th character, assumes a bug, and "line 4" that means
// "editor line 5" is a cross-referencing papercut. Even LSP, which is 0-based
// on the wire, is translated to 1-based by editors before display: 0-based
// human output has essentially no successful precedent.
//
// Col stays 0-based in storage to index []rune(Input) directly and align with
// Span's rune offsets; the 0-based machine offsets are exposed separately as
// start_char/stop_char for programmatic consumers that need to slice the input.
func (iss ParseIssue) DisplayCol() int {
	return iss.Col + 1
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
			iss.Line, iss.DisplayCol(), iss.Msg))
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
