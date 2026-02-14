package tablew

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// errorWriter implements output.ErrorWriter.
type errorWriter struct {
	w          io.Writer
	pr         *output.Printing
	stacktrace bool
}

// NewErrorWriter returns an output.ErrorWriter that
// outputs in text format.
func NewErrorWriter(w io.Writer, pr *output.Printing, stacktrace bool) output.ErrorWriter {
	return &errorWriter{w: w, pr: pr, stacktrace: stacktrace}
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(systemErr, humanErr error) {
	fmt.Fprintln(w.w, w.pr.Error.Sprintf("sq: %v", humanErr))
	if !w.stacktrace {
		return
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
