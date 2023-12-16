package jsonw

import (
	"fmt"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"io"
	"log/slog"
	"strings"

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

type errorDetail struct {
	Error     string   `json:"error,"`
	BaseError string   `json:"base_error,omitempty"`
	Stack     []*stack `json:"stack,omitempty"`
}

type stackError struct {
	Message string   `json:"msg"`
	Tree    []string `json:"tree,omitempty"`
}

type stack struct {
	Error *stackError `json:"error,omitempty"`
	Trace string      `json:"trace,omitempty"`
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(systemErr error, humanErr error) {
	pr := w.pr.Clone()
	pr.String = pr.Warning

	if !w.pr.Verbose {
		ed := errorDetail{Error: humanErr.Error()}
		_ = writeJSON(w.out, pr, ed)
		return
	}

	ed := errorDetail{
		Error:     humanErr.Error(),
		BaseError: systemErr.Error(),
	}

	stacks := errz.Stacks(systemErr)
	if len(stacks) > 0 {
		for _, sysStack := range stacks {
			if sysStack == nil {
				continue
			}

			st := &stack{
				Trace: strings.ReplaceAll(fmt.Sprintf("%+v", sysStack), "\n\t", "\n  "),
				Error: &stackError{
					Message: sysStack.Error.Error(),
					Tree:    stringz.TypeNames(errz.Chain(sysStack.Error)...),
				}}

			ed.Stack = append(ed.Stack, st)
		}
	}

	_ = writeJSON(w.out, pr, ed)
}
