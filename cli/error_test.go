package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli"
	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/testh"
)

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
