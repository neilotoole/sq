package jsonw

import (
	"context"
	"io"
	"time"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source"
)

var _ output.StmtExecWriter = (*stmtExecWriter)(nil)

type stmtExecWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewStmtExecWriter returns an output.StmtExecWriter.
func NewStmtExecWriter(out io.Writer, pr *output.Printing) output.StmtExecWriter {
	return &stmtExecWriter{
		out: out,
		pr:  pr,
	}
}

// StmtExecuted implements output.StmtExecWriter.
func (w *stmtExecWriter) StmtExecuted(_ context.Context, target *source.Source, affected int64, _ time.Duration) error {
	type stmtOutput struct {
		Target       string `json:"target" yaml:"target"`
		RowsAffected int64  `json:"rows_affected" yaml:"rows_affected"`
	}

	return writeJSON(w.out, w.pr, stmtOutput{
		Target:       target.Handle,
		RowsAffected: affected,
	})
}
