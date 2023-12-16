package tablew

import (
	"bytes"
	"fmt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"io"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// errorWriter implements output.ErrorWriter.
type errorWriter struct {
	w  io.Writer
	pr *output.Printing
}

// NewErrorWriter returns an output.ErrorWriter that
// outputs in text format.
func NewErrorWriter(w io.Writer, pr *output.Printing) output.ErrorWriter {
	return &errorWriter{w: w, pr: pr}
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(systemErr error, humanErr error) {
	fmt.Fprintln(w.w, w.pr.Error.Sprintf("sq: %v", humanErr))
	if !w.pr.Verbose {
		return
	}

	stacks := errz.Stacks(systemErr)
	if len(stacks) == 0 {
		return
	}

	var buf = &bytes.Buffer{}
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
			errTypes := stringz.TypeNames(errz.Chain(stack.Error)...)
			for i, typ := range errTypes {
				w.pr.StackErrorType.Fprint(buf, typ)
				if i < len(errTypes)-1 {
					w.pr.Faint.Fprint(buf, ":")
					buf.WriteByte(' ')
				}
			}
			buf.WriteByte('\n')
			w.pr.StackError.Fprintln(buf, stack.Error.Error())
		}

		lines := strings.Split(stackPrint, "\n")
		for _, line := range lines {
			w.pr.Stack.Fprint(buf, line)
			buf.WriteByte('\n')
		}
		count++
	}

	buf.WriteTo(w.w)
}
