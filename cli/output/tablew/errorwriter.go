package tablew

import (
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/output"
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
}
