package commonw_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/commonw"
	"github.com/neilotoole/sq/libsq/ast"
)

func newMonoPrinting() *output.Printing {
	pr := output.NewPrinting()
	pr.EnableColor(false) // deterministic output for tests
	return pr
}

func TestRenderParseError_SingleIssue(t *testing.T) {
	pe := &ast.ParseError{
		Input: ".actor | this_is_invalid(.first_name)",
		Issues: []ast.ParseIssue{
			{
				Stage:     "parser",
				Line:      1,
				Col:       8,
				StartChar: 8,
				StopChar:  22,
				Token:     "this_is_invalid",
				Msg:       "unexpected 'this_is_invalid'",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)

	got := buf.String()
	require.Contains(t, got, "syntax error at line 1, col 9: unexpected 'this_is_invalid'")
	require.Contains(t, got, ".actor | this_is_invalid(.first_name)")
	require.Contains(t, got, "~~~~~~~~~~~~~~~",
		"expected caret line marking the offending span")
}

func TestRenderParseError_WithSuggestion(t *testing.T) {
	pe := &ast.ParseError{
		Input: ".actor | mx(.id)",
		Issues: []ast.ParseIssue{
			{
				Stage:      "parser",
				Line:       1,
				Col:        8,
				StartChar:  8,
				StopChar:   9,
				Token:      "mx",
				Msg:        "unexpected 'mx'",
				Suggestion: "max",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)
	require.Contains(t, buf.String(), "did you mean 'max'?")
}

func TestRenderParseError_NoSpan(t *testing.T) {
	// Lexer error: StartChar == -1. Should still render a usable message.
	pe := &ast.ParseError{
		Input: ".actor # bad",
		Issues: []ast.ParseIssue{
			{
				Stage:     "lexer",
				Line:      1,
				Col:       7,
				StartChar: -1,
				StopChar:  -1,
				Token:     "",
				Msg:       "unexpected '#'",
			},
		},
	}
	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)

	got := buf.String()
	require.Contains(t, got, "syntax error at line 1, col 8: unexpected '#'")
	require.Contains(t, got, ".actor # bad")
}
