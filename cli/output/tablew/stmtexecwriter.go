package tablew

import (
	"context"
	"fmt"
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
func (w *stmtExecWriter) StmtExecuted(_ context.Context, affected int64, elapsed time.Duration) error {
	s := w.pr.Number.Sprintf("%d", affected)

	if affected == 1 {
		s += w.pr.Normal.Sprint(" row affected")
	} else {
		s += w.pr.Normal.Sprint(" rows affected")
	}

	if w.pr.Verbose {
		s += w.pr.Faint.Sprintf(" in %v", elapsed.Round(time.Millisecond))
	}

	_, err := fmt.Fprintln(w.out, s)
	return err
}
