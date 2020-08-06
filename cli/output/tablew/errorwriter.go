package tablew

import (
	"fmt"
	"io"

	"github.com/neilotoole/sq/cli/output"
)

// errorWriter implements output.ErrorWriter.
type errorWriter struct {
	w io.Writer
	f *output.Formatting
}

// NewErrorWriter returns an output.ErrorWriter that
// outputs in text format.
func NewErrorWriter(w io.Writer, f *output.Formatting) output.ErrorWriter {
	return &errorWriter{w: w, f: f}
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(err error) {
	fmt.Fprintln(w.w, w.f.Error.Sprintf("sq: %v", err))
}
