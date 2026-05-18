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
				Line:      1,
				Col:       9,
				StartChar: 9,
				StopChar:  12,
				Token:     "bad1",
				Msg:       "unexpected 'bad1'",
			},
			{
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

	// Verify the issues are separated by a blank line (the inter-issue
	// separator in RenderParseError). This guards against the separator
	// being silently removed.
	firstHdr := strings.Index(got, "unexpected 'bad1'")
	secondHdr := strings.Index(got, "unexpected 'bad2'")
	require.NotEqual(t, -1, firstHdr)
	require.NotEqual(t, -1, secondHdr)
	between := got[firstHdr:secondHdr]
	require.Contains(t, between, "\n\nsq:", "expected blank-line separator before second issue header")
}

func TestRenderParseError_NegativeSpanFallback(t *testing.T) {
	// Defensive: StartChar == -1 with no Token. Should still render a usable
	// message. Real lexer errors now synthesize a Token (see A3), but this
	// exercises the renderer's fallback path.
	pe := &ast.ParseError{
		Input: ".actor # bad",
		Issues: []ast.ParseIssue{
			{
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

func TestRenderParseError_MultiLineInput(t *testing.T) {
	pe := &ast.ParseError{
		Input: ".actor | bad\n.director | also_bad",
		Issues: []ast.ParseIssue{
			{
				Line:      1,
				Col:       9,
				StartChar: 9,
				StopChar:  11,
				Token:     "bad",
				Msg:       "unexpected 'bad'",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)
	got := buf.String()

	require.Contains(t, got, "syntax error at line 1, col 10: unexpected 'bad'")
	require.Contains(t, got, ".actor | bad")
	// Multi-line fallback still emits the caret line.
	require.Contains(t, got, "~~~")
	// The second input line should NOT appear in the rendered output
	// (we render the offending line only).
	require.NotContains(t, got, "also_bad")
}

func TestRenderParseError_MutesStringQuotes(t *testing.T) {
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = true })

	input := `@sakila/local/sl3 | .actor | gibberish | ".first_name"`
	pe := &ast.ParseError{
		Input: input,
		Issues: []ast.ParseIssue{
			{
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

	// Locate the inner content of the string token. When color is on the
	// quote characters carry their own SGR codes, so the literal
	// `".first_name"` won't appear as a contiguous substring; search for
	// the inner text instead.
	innerIdx := strings.Index(out, ".first_name")
	require.NotEqual(t, -1, innerIdx, "inner string token content not found in output")

	// The byte immediately before the opening `"` must be an ANSI SGR
	// terminator (`m`) — i.e., we just emitted a color escape for the
	// faint quote. Same for the inner-content boundary and the closing
	// quote. Easier to check: confirm pr.Faint's ANSI bytes appear near
	// the string token.
	faintEsc := pr.Faint.Sprint("")
	stringEsc := pr.String.Sprint("")
	require.NotEmpty(t, faintEsc, "pr.Faint should emit a non-empty SGR sequence when color is on")
	require.NotEmpty(t, stringEsc, "pr.String should emit a non-empty SGR sequence when color is on")

	// Region around the string token must contain BOTH faint and string
	// escape codes (proving we emit the quotes and the inner content with
	// different colors). Use generous slack to capture all SGR codes for
	// the surrounding quote characters.
	regionEnd := innerIdx + len(".first_name") + 64 // slack for closing quote SGR codes
	if regionEnd > len(out) {
		regionEnd = len(out)
	}
	region := out[max(0, innerIdx-64):regionEnd]
	// Extract the SGR-code substring of pr.Faint and pr.String (between
	// the `\x1b[` prefix and the `m` terminator).
	faintCode := sgrCode(faintEsc)
	stringCode := sgrCode(stringEsc)
	require.NotEqual(t, faintCode, stringCode,
		"pr.Faint and pr.String must differ (otherwise the test can't tell them apart)")
	require.Contains(t, region, faintCode, "expected pr.Faint SGR around string quote characters")
	require.Contains(t, region, stringCode, "expected pr.String SGR inside string content")
}

// sgrCode extracts the parameter portion of a single SGR escape
// sequence (e.g. "\x1b[2m" -> "[2m"). Used by the test to compare
// pr.Faint vs pr.String escape codes without depending on the exact
// numeric SGR values.
func sgrCode(s string) string {
	i := strings.Index(s, "\x1b")
	if i == -1 {
		return ""
	}
	j := strings.Index(s[i:], "m")
	if j == -1 {
		return s[i:]
	}
	return s[i : i+j+1]
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
