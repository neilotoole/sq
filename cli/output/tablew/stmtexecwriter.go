package tablew

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/driver/dialect"
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
func (w *stmtExecWriter) StmtExecuted(_ context.Context, _ *source.Source,
	affected int64, elapsed time.Duration,
) error {
	var s string
	switch affected {
	case dialect.RowsAffectedUnsupported:
		s = w.pr.Faint.Sprint("rows affected: unsupported")
	case 1:
		s = w.pr.Number.Sprintf("%d", affected)
		s += w.pr.Normal.Sprint(" row affected")
	default:
		s = w.pr.Number.Sprintf("%d", affected)
		s += w.pr.Normal.Sprint(" rows affected")
	}

	if w.pr.Verbose {
		s += w.pr.Faint.Sprintf(" in %v", elapsed.Round(time.Millisecond))
	}

	_, err := fmt.Fprintln(w.out, s)
	return err
}
