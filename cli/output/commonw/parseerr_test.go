package commonw_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
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

func newColorPrinting() *output.Printing {
	pr := output.NewPrinting()
	pr.EnableColor(true)
	return pr
}

func TestRenderParseError_SingleIssue(t *testing.T) {
	pe := &ast.ParseError{
		Input: ".actor | this_is_invalid(.first_name)",
		Issues: []ast.ParseIssue{
			{
				Stage:     "parser",
				Line:      1,
				Col:       9,
				StartChar: 9,
				StopChar:  23,
				Token:     "this_is_invalid",
				Msg:       "unexpected 'this_is_invalid'",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)

	got := buf.String()
	require.Contains(t, got, "syntax error at line 1, col 10: unexpected 'this_is_invalid'")
	require.Contains(t, got, ".actor | this_is_invalid(.first_name)")
	require.Contains(t, got, "this_is_invalid")
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
				Col:        9,
				StartChar:  9,
				StopChar:   10,
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

func TestRenderParseError_MultipleIssues(t *testing.T) {
	pe := &ast.ParseError{
		Input: ".actor | bad1 | bad2",
		Issues: []ast.ParseIssue{
			{
				Stage:     "parser",
				Line:      1,
				Col:       9,
				StartChar: 9,
				StopChar:  12,
				Token:     "bad1",
				Msg:       "unexpected 'bad1'",
			},
			{
				Stage:     "parser",
				Line:      1,
				Col:       16,
				StartChar: 16,
				StopChar:  19,
				Token:     "bad2",
				Msg:       "unexpected 'bad2'",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)

	got := buf.String()
	require.Contains(t, got, "unexpected 'bad1'")
	require.Contains(t, got, "unexpected 'bad2'")
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

func TestRenderParseError_ColorizesHandle(t *testing.T) {
	// Force color rendering even when the env says no terminal.
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = true }) // restore for other tests

	input := "@sakila/local/sl3 | .actor | gibberish"
	pe := &ast.ParseError{
		Input: input,
		Issues: []ast.ParseIssue{
			{
				Stage:     "parser",
				Line:      1,
				Col:       29,
				StartChar: 29,
				StopChar:  37,
				Token:     "gibberish",
				Msg:       "unexpected 'gibberish'",
			},
		},
	}

	pr := newColorPrinting()
	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, pr, pe)
	out := buf.String()

	// The input line is emitted with a 2-space indent followed immediately
	// by an ANSI SGR escape, then the handle text. Verify that:
	//   1. The handle text appears somewhere in the output.
	//   2. The two characters immediately before the handle's "@" are not
	//      plain spaces — i.e., an ANSI SGR code sits between the indent
	//      and the "@", proving the handle is colorized.
	const handleText = "@sakila/local/sl3"
	handleStart := strings.Index(out, handleText)
	require.NotEqual(t, -1, handleStart, "handle text not found in output")

	// The byte immediately before "@" must be part of an ANSI reset/color
	// sequence (e.g., "m" from "\x1b[34m"), not a plain space.
	require.NotEqual(t, byte(' '), out[handleStart-1],
		"expected an ANSI SGR byte immediately before '@', not a space: "+
			"handle does not appear to be colorized")
}
