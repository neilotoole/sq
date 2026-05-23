package tablew_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/tablew"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func TestErrorWriter_ParseError(t *testing.T) {
	pe := &ast.ParseError{
		Input: ".actor | this_is_invalid(.first_name)",
		Issues: []ast.ParseIssue{
			{
				Line:  1,
				Col:   9,
				Span:  &ast.Span{Start: 9, Stop: 23},
				Token: "this_is_invalid",
				Msg:   "unexpected 'this_is_invalid'",
			},
		},
	}
	wrapped := errz.Err(pe)

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := tablew.NewErrorWriter(buf, pr, false, true)
	w.Error(wrapped, wrapped)

	got := buf.String()
	require.Contains(t, got, "syntax error at line 1, col 10")
	require.Contains(t, got, ".actor | this_is_invalid(.first_name)")
	require.Contains(t, got, "~~~~~~~~~~~~~~~")
}

func TestErrorWriter_ParseError_EmptyIssuesFallsBack(t *testing.T) {
	// A *ast.ParseError with no Issues must not yield empty output:
	// RenderParseError is a no-op for empty Issues, so the writer must fall
	// back to the generic "sq: <err>" print.
	pe := &ast.ParseError{Input: ".actor", Issues: nil}
	wrapped := errz.Err(pe)

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := tablew.NewErrorWriter(buf, pr, false, true)
	w.Error(wrapped, wrapped)

	got := buf.String()
	require.NotEmpty(t, got, "empty-Issues ParseError must still produce output")
	require.Contains(t, got, "sq:", "must fall back to the generic error print")
}

func TestErrorWriter_NonParseError(t *testing.T) {
	// Generic errors should still print as before.
	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := tablew.NewErrorWriter(buf, pr, false, true)
	err := errors.New("something broke")
	w.Error(err, err)
	require.Contains(t, buf.String(), "sq: something broke")
}

func TestErrorWriter_ParseError_StacktraceHonored(t *testing.T) {
	pe := &ast.ParseError{
		Input: ".actor | bad",
		Issues: []ast.ParseIssue{
			{
				Line:  1,
				Col:   9,
				Span:  &ast.Span{Start: 9, Stop: 11},
				Token: "bad",
				Msg:   "unexpected 'bad'",
			},
		},
	}
	wrapped := errz.Err(pe)

	// With stacktrace=false: no stack frames.
	bufOff := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	wOff := tablew.NewErrorWriter(bufOff, pr, false, true)
	wOff.Error(wrapped, wrapped)
	require.Contains(t, bufOff.String(), "syntax error")
	require.NotContains(t, bufOff.String(), "goroutine")

	// With stacktrace=true: parse error rendering PLUS stack frames.
	bufOn := &bytes.Buffer{}
	wOn := tablew.NewErrorWriter(bufOn, pr, true, true)
	wOn.Error(wrapped, wrapped)
	require.Contains(t, bufOn.String(), "syntax error",
		"parse error message should appear above the stack")
	require.NotEqual(t, bufOff.String(), bufOn.String(),
		"stacktrace=true should produce different output than stacktrace=false")
}

func TestErrorWriter_ParseError_NonVerbose(t *testing.T) {
	// With verbose=false, only the one-line summary is shown: no input line,
	// caret, or "did you mean" suggestion.
	pe := &ast.ParseError{
		Input: ".actor | mx(.first_name)",
		Issues: []ast.ParseIssue{
			{
				Line:       1,
				Col:        9,
				Span:       &ast.Span{Start: 9, Stop: 10},
				Token:      "mx",
				Msg:        "unexpected 'mx'",
				Suggestion: "max",
			},
		},
	}
	wrapped := errz.Err(pe)

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := tablew.NewErrorWriter(buf, pr, false, false)
	w.Error(wrapped, wrapped)

	got := buf.String()
	require.Contains(t, got, "sq: syntax error at line 1, col 10: unexpected 'mx'")
	require.NotContains(t, got, "did you mean", "suggestion must be omitted when not verbose")
	require.NotContains(t, got, "~", "caret line must be omitted when not verbose")
	require.NotContains(t, got, ".actor | mx", "input line must be omitted when not verbose")
}
