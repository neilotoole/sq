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
				Line:      1,
				Col:       9,
				StartChar: 9,
				StopChar:  23,
				Token:     "this_is_invalid",
				Msg:       "unexpected 'this_is_invalid'",
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
	require.Equal(t, 9, got.ParseError.Issues[0].StartChar)
	require.Equal(t, 23, got.ParseError.Issues[0].StopChar)
}

func TestJSONErrorWriter_ParseError_Verbose(t *testing.T) {
	pe := &ast.ParseError{
		Input: ".actor | bad",
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
