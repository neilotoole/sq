package jsonw

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// errorWriter implements output.ErrorWriter.
type errorWriter struct {
	log *slog.Logger
	out io.Writer
	pr  *output.Printing
}

// NewErrorWriter returns an output.ErrorWriter that outputs in JSON.
func NewErrorWriter(log *slog.Logger, out io.Writer, pr *output.Printing) output.ErrorWriter {
	return &errorWriter{log: log, out: out, pr: pr}
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(systemErr error, humanErr error) {
	var errMsg string
	var stack []string

	if systemErr == nil {
		errMsg = "nil error"
	} else {
		errMsg = systemErr.Error()
		if w.pr.Verbose {
			for _, st := range errz.Stacks(systemErr) {
				s := fmt.Sprintf("%+v", st)
				stack = append(stack, s)
			}
		}
	}

	t := struct {
		Error string   `json:"error"`
		Stack []string `json:"stack,omitempty"`
	}{
		Error: errMsg,
		Stack: stack,
	}

	pr := w.pr.Clone()
	pr.String = pr.Error

	_ = writeJSON(w.out, pr, t)
}
