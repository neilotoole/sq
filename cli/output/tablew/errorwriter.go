package tablew

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/commonw"
	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// errorWriter implements output.ErrorWriter.
type errorWriter struct {
	w          io.Writer
	pr         *output.Printing
	stacktrace bool
	verbose    bool
}

// NewErrorWriter returns an output.ErrorWriter that outputs in text format.
// When verbose is true, an SLQ parse error is rendered in full (highlighted
// span plus any "did you mean" suggestion); when false, only the one-line
// summary is shown. See the error.format.text.verbose option.
func NewErrorWriter(w io.Writer, pr *output.Printing, stacktrace, verbose bool) output.ErrorWriter {
	return &errorWriter{w: w, pr: pr, stacktrace: stacktrace, verbose: verbose}
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(systemErr, humanErr error) {
	var pe *ast.ParseError
	if errors.As(systemErr, &pe) && len(pe.Issues) > 0 {
		if w.verbose {
			commonw.RenderParseError(w.w, w.pr, pe)
		} else {
			commonw.RenderParseErrorSummary(w.w, w.pr, pe)
		}
		// Render the stack trace below only when --error.stack is set.
		// ANTLR internals aren't useful, but the wrapping errz frames may be.
		if !w.stacktrace {
			return
		}
	} else {
		fmt.Fprintln(w.w, w.pr.Error.Sprintf("sq: %v", humanErr))
		if !w.stacktrace {
			return
		}
	}

	stacks := errz.Stacks(systemErr)
	if len(stacks) == 0 {
		return
	}

	buf := &bytes.Buffer{}
	var count int
	for _, stack := range stacks {
		if stack == nil {
			continue
		}

		stackPrint := fmt.Sprintf("%+v", stack.Frames)
		stackPrint = strings.ReplaceAll(strings.TrimSpace(stackPrint), "\n\t", "\n  ")
		if stackPrint == "" {
			continue
		}

		if count > 0 {
			buf.WriteString("\n")
		}

		if stack.Error != nil {
			w.pr.StackErrorType.Fprintln(buf, errz.SprintTreeTypes(stack.Error))
			w.pr.StackError.Fprintln(buf, stack.Error.Error())
		}

		lines := strings.SplitSeq(stackPrint, "\n")
		for line := range lines {
			w.pr.Stack.Fprint(buf, line)
			buf.WriteByte('\n')
		}
		count++
	}

	_, _ = buf.WriteTo(w.w)
}
