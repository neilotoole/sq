package jsonw_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
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
