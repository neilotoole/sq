package tablew

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/source"
)

var _ output.RecordInsertWriter = (*recordInsertWriter)(nil)

type recordInsertWriter struct {
	out io.Writer
	pr  *output.Printing
}

// NewRecordInsertWriter returns an output.RecordInsertWriter.
func NewRecordInsertWriter(out io.Writer, pr *output.Printing) output.RecordInsertWriter {
	return &recordInsertWriter{
		out: out,
		pr:  pr,
	}
}

// RecordsInserted implements output.RecordInsertWriter.
func (w *recordInsertWriter) RecordsInserted(_ context.Context, target *source.Source, tbl string,
	rowsInserted int64, elapsed time.Duration,
) error {
	s := w.pr.Number.Sprintf("%d", rowsInserted)

	if rowsInserted == 1 {
		s += w.pr.Normal.Sprint(" row inserted into ")
	} else {
		s += w.pr.Normal.Sprint(" rows inserted into ")
	}

	s += w.pr.Handle.Sprint(source.Target(target, tbl))

	if w.pr.Verbose {
		s += w.pr.Faint.Sprintf(" in %v", elapsed.Round(time.Millisecond))
	}

	_, err := fmt.Fprintln(w.out, s)
	return err
}
