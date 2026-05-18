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
	w := tablew.NewErrorWriter(buf, pr, false)
	w.Error(wrapped, wrapped)

	got := buf.String()
	require.Contains(t, got, "syntax error at line 1, col 10")
	require.Contains(t, got, ".actor | this_is_invalid(.first_name)")
	require.Contains(t, got, "~~~~~~~~~~~~~~~")
}

func TestErrorWriter_NonParseError(t *testing.T) {
	// Generic errors should still print as before.
	buf := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	w := tablew.NewErrorWriter(buf, pr, false)
	err := errors.New("something broke")
	w.Error(err, err)
	require.Contains(t, buf.String(), "sq: something broke")
}

func TestErrorWriter_ParseError_StacktraceHonored(t *testing.T) {
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

	// With stacktrace=false: no stack frames.
	bufOff := &bytes.Buffer{}
	pr := output.NewPrinting()
	pr.EnableColor(false)
	wOff := tablew.NewErrorWriter(bufOff, pr, false)
	wOff.Error(wrapped, wrapped)
	require.Contains(t, bufOff.String(), "syntax error")
	require.NotContains(t, bufOff.String(), "goroutine")

	// With stacktrace=true: parse error rendering PLUS stack frames.
	bufOn := &bytes.Buffer{}
	wOn := tablew.NewErrorWriter(bufOn, pr, true)
	wOn.Error(wrapped, wrapped)
	require.Contains(t, bufOn.String(), "syntax error",
		"parse error message should appear above the stack")
	require.NotEqual(t, bufOff.String(), bufOn.String(),
		"stacktrace=true should produce different output than stacktrace=false")
}
