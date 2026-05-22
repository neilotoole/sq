package jsonw_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// testParseIssueJSON mirrors the JSON wire shape of a parse issue.
type testParseIssueJSON struct {
	Line       int    `json:"line"`
	Col        int    `json:"col"`
	StartChar  int    `json:"start_char"`
	StopChar   int    `json:"stop_char"`
	Token      string `json:"token"`
	Msg        string `json:"msg"`
	Suggestion string `json:"suggestion,omitempty"`
}

// testParseErrorJSON mirrors the JSON wire shape of a parse error.
type testParseErrorJSON struct {
	Input  string               `json:"input"`
	Issues []testParseIssueJSON `json:"issues"`
}

// testErrorDetailJSON mirrors the JSON wire shape of an error response.
type testErrorDetailJSON struct {
	ParseError *testParseErrorJSON `json:"parse_error"`
	Error      string              `json:"error"`
}

func TestJSONErrorWriter_ParseError(t *testing.T) {
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
	w := jsonw.NewErrorWriter(slog.Default(), buf, pr)
	w.Error(wrapped, wrapped)

	var got testErrorDetailJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.NotNil(t, got.ParseError)
	require.Equal(t, pe.Input, got.ParseError.Input)
	require.Len(t, got.ParseError.Issues, 1)
	require.Equal(t, "this_is_invalid", got.ParseError.Issues[0].Token)
	// line/col are 1-based human coordinates: a 0-based Col of 9 surfaces as
	// col 10, matching the text error output.
	require.Equal(t, 1, got.ParseError.Issues[0].Line)
	require.Equal(t, 10, got.ParseError.Issues[0].Col)
	// start_char/stop_char stay 0-based rune offsets, so start_char (9) is
	// deliberately one less than col (10) for the same position.
	require.Equal(t, 9, got.ParseError.Issues[0].StartChar)
	require.Equal(t, 23, got.ParseError.Issues[0].StopChar)
}

func TestJSONErrorWriter_ParseError_NoSpan(t *testing.T) {
	// A ParseIssue with nil Span must omit start_char/stop_char from the
	// wire form rather than emit a sentinel value.
	pe := &ast.ParseError{
		Input: ".actor # bad",
		Issues: []ast.ParseIssue{
			{Line: 1, Col: 7, Msg: "unexpected '#'"},
		},
	}
	wrapped := errz.Err(pe)

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := jsonw.NewErrorWriter(slog.Default(), buf, pr)
	w.Error(wrapped, wrapped)

	raw := buf.String()
	require.NotContains(t, raw, "start_char", "nil Span must omit start_char")
	require.NotContains(t, raw, "stop_char", "nil Span must omit stop_char")
	require.Contains(t, raw, `"col"`, "col is always present")

	// Even with no span, the 1-based human col must still be reported.
	var got testErrorDetailJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, 1, got.ParseError.Issues[0].Line)
	require.Equal(t, 8, got.ParseError.Issues[0].Col,
		"Col 7 (0-based) must surface as col 8 even when no span is emitted")
}

func TestJSONErrorWriter_ParseError_Suggestion(t *testing.T) {
	// The did-you-mean suggestion must reach the JSON wire form.
	pe := &ast.ParseError{
		Input: ".actor | mx(.id)",
		Issues: []ast.ParseIssue{
			{Line: 1, Col: 9, Span: &ast.Span{Start: 9, Stop: 10}, Token: "mx", Msg: "unexpected 'mx'", Suggestion: "max"},
		},
	}
	wrapped := errz.Err(pe)

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := jsonw.NewErrorWriter(slog.Default(), buf, pr)
	w.Error(wrapped, wrapped)

	var got testErrorDetailJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.NotNil(t, got.ParseError)
	require.Equal(t, "max", got.ParseError.Issues[0].Suggestion)
}

func TestJSONErrorWriter_ParseError_EmptySpanOmitted(t *testing.T) {
	// The <EOF> token yields an empty span (Stop < Start). The wire form must
	// omit start_char/stop_char rather than serialize a backwards range.
	pe := &ast.ParseError{
		Input: ".actor |",
		Issues: []ast.ParseIssue{
			{Line: 1, Col: 8, Span: &ast.Span{Start: 8, Stop: 7}, Token: "<EOF>", Msg: "unexpected end of input"},
		},
	}
	wrapped := errz.Err(pe)

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := jsonw.NewErrorWriter(slog.Default(), buf, pr)
	w.Error(wrapped, wrapped)

	raw := buf.String()
	require.NotContains(t, raw, "start_char", "empty (EOF) span must omit start_char")
	require.NotContains(t, raw, "stop_char", "empty (EOF) span must omit stop_char")
	require.Contains(t, raw, `"col"`, "col is always present")

	// The empty-span <EOF> case still reports a 1-based human col.
	var got testErrorDetailJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Equal(t, 9, got.ParseError.Issues[0].Col,
		"Col 8 (0-based) must surface as col 9 for the <EOF> empty-span case")
}

func TestJSONErrorWriter_ParseError_MixedSpans(t *testing.T) {
	// Multiple issues with DISTINCT spans must each serialize their own
	// offsets (guards against the loop-local pointers aliasing to the last
	// issue), and an issue with no span must omit start_char/stop_char.
	pe := &ast.ParseError{
		Input: ".actor | bad1 | bad2 |",
		Issues: []ast.ParseIssue{
			{Line: 1, Col: 9, Span: &ast.Span{Start: 9, Stop: 23}, Token: "bad1", Msg: "unexpected 'bad1'"},
			{Line: 1, Col: 30, Span: &ast.Span{Start: 30, Stop: 40}, Token: "bad2", Msg: "unexpected 'bad2'"},
			{Line: 1, Col: 50, Msg: "unexpected end of input"},
		},
	}
	wrapped := errz.Err(pe)

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := jsonw.NewErrorWriter(slog.Default(), buf, pr)
	w.Error(wrapped, wrapped)

	var got testErrorDetailJSON
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.Len(t, got.ParseError.Issues, 3)
	// Each span issue serializes its own offsets, not the last issue's.
	require.Equal(t, 9, got.ParseError.Issues[0].StartChar)
	require.Equal(t, 23, got.ParseError.Issues[0].StopChar)
	require.Equal(t, 30, got.ParseError.Issues[1].StartChar)
	require.Equal(t, 40, got.ParseError.Issues[1].StopChar)
	// The no-span issue omits the offsets entirely.
	require.Equal(t, 2, strings.Count(buf.String(), `"start_char"`),
		"only the two issues with spans should emit start_char")
}

func TestJSONErrorWriter_ParseError_Verbose(t *testing.T) {
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

	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	pr.Verbose = true
	w := jsonw.NewErrorWriter(slog.Default(), buf, pr)
	w.Error(wrapped, wrapped)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	require.NotNil(t, got["parse_error"], "parse_error field must appear in verbose output")
	require.NotEmpty(t, got["base_error"], "verbose output must include base_error")
	require.NotEmpty(t, got["tree"], "verbose output must include tree")

	// Lock the JSON key order: error must precede parse_error.
	// encoding/json marshals fields in declaration order; if errorDetail
	// is ever reordered (e.g. by an unannotated fieldalignment pass),
	// the visible key order would change. Pin it here.
	raw := buf.String()
	errIdx := strings.Index(raw, `"error"`)
	peIdx := strings.Index(raw, `"parse_error"`)
	require.NotEqual(t, -1, errIdx)
	require.NotEqual(t, -1, peIdx)
	require.Less(t, errIdx, peIdx, "error key must precede parse_error in JSON output")
}
