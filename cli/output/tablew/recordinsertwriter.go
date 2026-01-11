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
func (w *recordInsertWriter) RecordsInserted(_ context.Context, destSrc *source.Source, destTbl string,
	rowsInserted int64, elapsed time.Duration,
) error {
	s := w.pr.String.Sprintf("Inserted ") +
		w.pr.Number.Sprintf("%d", rowsInserted)

	if rowsInserted == 1 {
		s += w.pr.String.Sprint(" row")
	} else {
		s += w.pr.String.Sprint(" rows")
	}

	s += w.pr.String.Sprint(" into ") +
		w.pr.Handle.Sprint(destSrc.Handle) +
		w.pr.Faint.Sprint(".") +
		w.pr.String.Sprint(destTbl)

	if w.pr.Verbose {
		s += w.pr.Faint.Sprintf(" in %v", elapsed.Round(time.Millisecond))
	}

	fmt.Fprintln(w.out, s)
	return nil
}