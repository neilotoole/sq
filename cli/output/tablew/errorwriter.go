package tablew

import (
	"fmt"
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
func (w *errorWriter) Error(err error) {
	fmt.Fprintln(w.w, w.pr.Error.Sprintf("sq: %v", err))
	if !w.pr.Verbose {
		return
	}

	stacks := errz.Stack(err)
	for i, stack := range stacks {
		if i > 0 {
			fmt.Fprintln(w.w)
		}

		s := fmt.Sprintf("%+v", stack)
		s = strings.TrimSpace(s)
		w.pr.Faint.Fprintln(w.w, s)
	}
}
