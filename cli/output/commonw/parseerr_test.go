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
	"github.com/neilotoole/sq/libsq/source/metadata"
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
				Line:  1,
				Col:   9,
				Span:  &ast.Span{Start: 9, Stop: 23},
				Token: "this_is_invalid",
				Msg:   "unexpected 'this_is_invalid'",
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
				Span:       &ast.Span{Start: 9, Stop: 10},
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
				Line:  1,
				Col:   9,
				Span:  &ast.Span{Start: 9, Stop: 12},
				Token: "bad1",
				Msg:   "unexpected 'bad1'",
			},
			{
				Line:  1,
				Col:   16,
				Span:  &ast.Span{Start: 16, Stop: 19},
				Token: "bad2",
				Msg:   "unexpected 'bad2'",
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

func TestRenderParseError_NoSpanFallback(t *testing.T) {
	// Defensive: nil Span with no Token. Should still render a usable message
	// from Line/Col. Real lexer errors now synthesize a Token and Span, but
	// this exercises the renderer's fallback path.
	pe := &ast.ParseError{
		Input: ".actor # bad",
		Issues: []ast.ParseIssue{
			{
				Line: 1,
				Col:  7,
				Msg:  "unexpected '#'",
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
				Line:  1,
				Col:   9,
				Span:  &ast.Span{Start: 9, Stop: 11},
				Token: "bad",
				Msg:   "unexpected 'bad'",
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

// caretLine returns the first line of rendered output containing a caret
// (the "~~~" run), or "" if none. Used to assert exact caret placement.
func caretLine(rendered string) string {
	for ln := range strings.SplitSeq(rendered, "\n") {
		if strings.Contains(ln, "~") {
			return ln
		}
	}
	return ""
}

func TestRenderParseError_MultiLineCaretOnLaterLine(t *testing.T) {
	// Short first line, error on line 2. Span carries ABSOLUTE rune offsets
	// (Start=11), but the caret must land under the token's line-local column
	// (9), matching the "col 10" header. Regression test for spanWithinLine
	// treating absolute offsets as line-local (caret shifted by the line's
	// start offset and truncated).
	pe := &ast.ParseError{
		Input: "a\n.actor | this_is_invalid",
		Issues: []ast.ParseIssue{
			{
				Line:  2,
				Col:   9,
				Span:  &ast.Span{Start: 11, Stop: 25},
				Token: "this_is_invalid",
				Msg:   "unexpected 'this_is_invalid'",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)
	got := buf.String()

	require.Contains(t, got, "syntax error at line 2, col 10:")
	// 2-space indent + 9 line-local columns = 11 spaces, then 15 tildes
	// covering "this_is_invalid".
	require.Equal(t, strings.Repeat(" ", 11)+strings.Repeat("~", 15), caretLine(got),
		"caret must sit under the token at its line-local column, not shifted by lineStart")
}

func TestRenderParseError_NonASCIICaret(t *testing.T) {
	// A multibyte rune ('é') precedes the offending span; the caret must be
	// placed by rune offset so it sits directly under "gibberish".
	pe := &ast.ParseError{
		Input: `.actor | "café" | gibberish`,
		Issues: []ast.ParseIssue{
			{
				Line:  1,
				Col:   18,
				Span:  &ast.Span{Start: 18, Stop: 26},
				Token: "gibberish",
				Msg:   "unexpected 'gibberish'",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)
	got := buf.String()

	// 2-space indent + 18 runes before "gibberish" = 20 spaces, then 9 tildes.
	require.Equal(t, strings.Repeat(" ", 20)+strings.Repeat("~", 9), caretLine(got),
		"caret must align by rune offset, not byte offset")
}

func TestRenderParseError_EOFCaret(t *testing.T) {
	// The synthetic <EOF> token has a zero-width span (Stop < Start). The
	// renderer must still emit a single caret at the end-of-input position
	// rather than suppressing the caret line.
	pe := &ast.ParseError{
		Input: ".actor |",
		Issues: []ast.ParseIssue{
			{
				Line:  1,
				Col:   8,
				Span:  &ast.Span{Start: 8, Stop: 7},
				Token: "<EOF>",
				Msg:   "unexpected end of input",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)
	got := buf.String()

	require.Contains(t, got, "unexpected end of input")
	// 2-space indent + 8 columns (length of ".actor |") = 10 spaces, then a
	// single caret at the end-of-input position.
	require.Equal(t, strings.Repeat(" ", 10)+"~", caretLine(got),
		"EOF should still produce a single caret at the end position")
}

func TestRenderParseError_MultiLineCaret_MultibytePrecedingLine(t *testing.T) {
	// Line 1 holds a multibyte rune ('é'); the error is on line 2. lineStart
	// must be computed in RUNES (len([]rune("café"))+1 = 5), not bytes (6),
	// or the line-2 caret shifts. Span.Start is the absolute rune offset (14)
	// of "gibberish".
	pe := &ast.ParseError{
		Input: "café\n.actor | gibberish",
		Issues: []ast.ParseIssue{
			{
				Line:  2,
				Col:   9,
				Span:  &ast.Span{Start: 14, Stop: 22},
				Token: "gibberish",
				Msg:   "unexpected 'gibberish'",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)
	got := buf.String()

	require.Contains(t, got, "syntax error at line 2, col 10:")
	// 2-space indent + 9 line-local columns = 11 spaces, then 9 tildes under
	// "gibberish". A byte-based lineStart would shift this left by one.
	require.Equal(t, strings.Repeat(" ", 11)+strings.Repeat("~", 9), caretLine(got),
		"lineStart must use the rune length of the preceding line, not bytes")
}

func TestRenderParseError_EOFCaret_LaterLine(t *testing.T) {
	// EOF on line 2: the empty span's absolute Start (10) must map to the
	// line-local end position via lineStart (2) and still emit a single caret.
	pe := &ast.ParseError{
		Input: "x\n.actor |",
		Issues: []ast.ParseIssue{
			{
				Line:  2,
				Col:   8,
				Span:  &ast.Span{Start: 10, Stop: 9},
				Token: "<EOF>",
				Msg:   "unexpected end of input",
			},
		},
	}

	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, newMonoPrinting(), pe)
	got := buf.String()

	require.Contains(t, got, "syntax error at line 2, col 9:")
	require.Equal(t, strings.Repeat(" ", 10)+"~", caretLine(got),
		"EOF caret on a later line must map to the line-local end position")
}

func TestRenderParseError_MutesStringQuotes(t *testing.T) {
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = true })

	input := `@sakila/local/sl3 | .actor | gibberish | ".first_name"`
	pe := &ast.ParseError{
		Input: input,
		Issues: []ast.ParseIssue{
			{
				Line:  1,
				Col:   29,
				Span:  &ast.Span{Start: 29, Stop: 37},
				Token: "gibberish",
				Msg:   "unexpected 'gibberish'",
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
	regionEnd := min(innerIdx+len(".first_name")+64, len(out)) // slack for closing quote SGR codes
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

func TestColumnKey(t *testing.T) {
	tbl := &metadata.Table{
		Name: "t",
		Columns: []*metadata.Column{
			{Name: "id", PrimaryKey: true},
			{Name: "ref"},
			{Name: "uq"},
		},
		FK:                &metadata.FKGroup{Outgoing: []*metadata.ForeignKey{{Columns: []string{"ref"}}}},
		UniqueConstraints: []*metadata.UniqueConstraint{{Columns: []string{"uq"}}},
	}
	fk := commonw.FKColumnSet(tbl)
	uc := commonw.UCColumnSet(tbl)
	require.Equal(t, "PK", commonw.ColumnKey(tbl.Columns[0], fk, uc))
	require.Equal(t, "FK", commonw.ColumnKey(tbl.Columns[1], fk, uc))
	require.Equal(t, "UK", commonw.ColumnKey(tbl.Columns[2], fk, uc))
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

func TestRenderParseError_MultiLineColorHilite(t *testing.T) {
	// Multi-line input uses the plain-text + ErrorHilite overlay path (not
	// per-token colorization). With color on, the offending span must be
	// wrapped in pr.ErrorHilite's SGR codes.
	color.NoColor = false
	t.Cleanup(func() { color.NoColor = true })

	pe := &ast.ParseError{
		Input: ".actor | bad\n.director | x",
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

	pr := newColorPrinting()
	buf := &bytes.Buffer{}
	commonw.RenderParseError(buf, pr, pe)
	out := buf.String()

	hiliteCode := sgrCode(pr.ErrorHilite.Sprint(""))
	require.NotEmpty(t, hiliteCode, "pr.ErrorHilite should emit an SGR sequence when color is on")
	require.Contains(t, out, hiliteCode,
		"multi-line offending span must be wrapped in pr.ErrorHilite SGR codes")
	require.Contains(t, out, "bad")
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
				Line:  1,
				Col:   29,
				Span:  &ast.Span{Start: 29, Stop: 37},
				Token: "gibberish",
				Msg:   "unexpected 'gibberish'",
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
