package jsonw

import (
	"context"
	"io"
	"time"

	"github.com/neilotoole/sq/cli/output"
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
func (w *stmtExecWriter) StmtExecuted(_ context.Context, affected int64, _ time.Duration) error {
	type stmtOutput struct {
		RowsAffected int64 `json:"rows_affected" yaml:"rows_affected"`
	}

	return writeJSON(w.out, w.pr, stmtOutput{
		RowsAffected: affected,
	})
}
