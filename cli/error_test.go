package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/testh"
)

// e2eErrorDetail and friends mirror the relevant subset of sq's JSON error
// wire form for the end-to-end test below.
type e2eErrorDetail struct {
	Error      string         `json:"error"`
	ParseError *e2eParseError `json:"parse_error"`
}

type e2eParseError struct {
	Issues []e2eParseIssue `json:"issues"`
}

type e2eParseIssue struct {
	Line int    `json:"line"`
	Col  int    `json:"col"`
	Msg  string `json:"msg"`
}

// TestPrintError_ParseError_EndToEnd runs a malformed SLQ query through the
// real CLI and asserts the rich, position-highlighted error reaches stderr
// in text mode, and the structured parse_error reaches --json output. This
// guards the full chain (query -> ast.Parse -> error propagation -> writer)
// that the other tests, which hand-build a ParseError, do not exercise.
func TestPrintError_ParseError_EndToEnd(t *testing.T) {
	const badQuery = ".actor | this_is_invalid(.first_name)"

	t.Run("text", func(t *testing.T) {
		th := testh.New(t)
		tr := testrun.New(th.Context, t, nil)
		require.Error(t, tr.Exec("slq", badQuery))
		got := tr.ErrOut.String()
		require.Contains(t, got, "syntax error at line 1, col 10: unexpected 'this_is_invalid'")
		require.Contains(t, got, "~~~", "expected the caret line in the rich render")
	})

	t.Run("json", func(t *testing.T) {
		// Error output format is controlled by error.format, independent of
		// the record-output --json/--format flag.
		th := testh.New(t)
		tr := testrun.New(th.Context, t, nil)
		require.Error(t, tr.Exec("slq", "--error.format=json", badQuery))

		var got e2eErrorDetail
		require.NoError(t, json.Unmarshal(tr.ErrOut.Bytes(), &got))
		require.NotNil(t, got.ParseError, "parse_error must be present in --json error output")
		require.NotEmpty(t, got.ParseError.Issues)
		require.Equal(t, 1, got.ParseError.Issues[0].Line, "JSON line is 1-based")
		require.Equal(t, 10, got.ParseError.Issues[0].Col,
			"JSON col is 1-based, matching the text output's 'col 10'")
		require.Contains(t, got.ParseError.Issues[0].Msg, "unexpected 'this_is_invalid'")
	})
}

// TestPrintError_ParseError_Bootstrap covers PrintError's bootstrap fallback
// (ru.Writers == nil), where a *ast.ParseError is rendered directly instead
// of via the run's error writer.
func TestPrintError_ParseError_Bootstrap(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Run.Writers = nil // force the bootstrap fallback path

	pe := &ast.ParseError{
		Input: ".actor | this_is_invalid",
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

	cli.PrintError(th.Context, tr.Run, errz.Err(pe))

	got := tr.ErrOut.String()
	require.Contains(t, got, "syntax error at line 1, col 10: unexpected 'this_is_invalid'")
	require.Contains(t, got, "~~~", "bootstrap path should still render the caret")
}

// TestPrintError_GenericError_Bootstrap asserts that a non-parse error in the
// bootstrap fallback also reaches errOut (not os.Stderr directly), so all
// three bootstrap branches (JSON, parse error, generic) write consistently.
func TestPrintError_GenericError_Bootstrap(t *testing.T) {
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Run.Writers = nil // force the bootstrap fallback path

	cli.PrintError(th.Context, tr.Run, errz.New("something broke"))

	require.Contains(t, tr.ErrOut.String(), "sq: something broke",
		"generic bootstrap error must reach errOut, consistent with the parse-error path")
}

// humanReadableError is a test double implementing errz.HumanReadable.
type humanReadableError struct{ human, full string }

func (e *humanReadableError) Error() string      { return e.full }
func (e *humanReadableError) HumanError() string { return e.human }

func TestHumanizeError_HumanReadable(t *testing.T) {
	inner := &humanReadableError{
		human: "short and friendly (see docs).",
		full:  "long diagnostic dump: tried all peers unsuccessfully...",
	}
	err := errz.Wrap(errz.Err(inner), "failed to read @rq source metadata")

	got := cli.HumanizeError(err)
	require.Equal(t, inner.human, got.Error(),
		"a HumanReadable in the chain must replace the full rendered chain")

	// Plain errors pass through with message intact.
	plain := errz.New("ordinary failure")
	require.Equal(t, "ordinary failure", cli.HumanizeError(plain).Error())
}
