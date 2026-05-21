package jsonw

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/ast"
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

type errorDetail struct { //nolint:govet // declaration order is the JSON output order
	Error      string          `json:"error"`
	BaseError  string          `json:"base_error,omitempty"`
	Tree       string          `json:"tree,omitempty"`
	Stack      []*stack        `json:"stack,omitempty"`
	ParseError *parseErrorJSON `json:"parse_error,omitempty"`
}

type parseIssueJSON struct { //nolint:govet // declaration order is the JSON output order
	Line       int    `json:"line"`
	Col        int    `json:"col"`
	StartChar  *int   `json:"start_char,omitempty"`
	StopChar   *int   `json:"stop_char,omitempty"`
	Token      string `json:"token,omitempty"`
	Msg        string `json:"msg"`
	Suggestion string `json:"suggestion,omitempty"`
}

type parseErrorJSON struct {
	Input  string           `json:"input"`
	Issues []parseIssueJSON `json:"issues"`
}

type stackError struct {
	Message string `json:"msg"`
	Tree    string `json:"tree,omitempty"`
}

type stack struct {
	Error *stackError `json:"error,omitempty"`
	Trace string      `json:"trace,omitempty"`
}

// Error implements output.ErrorWriter.
func (w *errorWriter) Error(systemErr, humanErr error) {
	pr := w.pr.Clone()
	pr.String = pr.Warning

	var pe *ast.ParseError
	hasParseErr := errors.As(systemErr, &pe)

	if !w.pr.Verbose {
		ed := errorDetail{Error: humanErr.Error()}
		if hasParseErr && len(pe.Issues) > 0 {
			ed.ParseError = toParseErrorJSON(pe)
		}
		_ = writeJSON(w.out, pr, ed)
		return
	}

	ed := errorDetail{
		Error:     humanErr.Error(),
		BaseError: systemErr.Error(),
		Tree:      errz.SprintTreeTypes(systemErr),
	}
	if hasParseErr && len(pe.Issues) > 0 {
		ed.ParseError = toParseErrorJSON(pe)
	}

	stacks := errz.Stacks(systemErr)
	if len(stacks) > 0 {
		for _, sysStack := range stacks {
			if sysStack == nil {
				continue
			}

			st := &stack{
				Trace: strings.ReplaceAll(fmt.Sprintf("%+v", sysStack.Frames), "\n\t", "\n  "),
				Error: &stackError{
					Message: sysStack.Error.Error(),
					Tree:    errz.SprintTreeTypes(sysStack.Error),
				},
			}

			ed.Stack = append(ed.Stack, st)
		}
	}

	_ = writeJSON(w.out, pr, ed)
}

// toParseErrorJSON converts a *ast.ParseError to the JSON wire form.
func toParseErrorJSON(pe *ast.ParseError) *parseErrorJSON {
	out := &parseErrorJSON{
		Input:  pe.Input,
		Issues: make([]parseIssueJSON, len(pe.Issues)),
	}
	for i, iss := range pe.Issues {
		ij := parseIssueJSON{
			Line:       iss.Line,
			Col:        iss.Col,
			Token:      iss.Token,
			Msg:        iss.Msg,
			Suggestion: iss.Suggestion,
		}
		if iss.Span != nil {
			start, stop := iss.Span.Start, iss.Span.Stop
			ij.StartChar, ij.StopChar = &start, &stop
		}
		out.Issues[i] = ij
	}
	return out
}
