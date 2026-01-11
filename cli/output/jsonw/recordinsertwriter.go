package jsonw

import (
	"context"
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
func (w *recordInsertWriter) RecordsInserted(_ context.Context, target *source.Source,
	tbl string, rowsInserted int64, _ time.Duration,
) error {
	type insertOutput struct {
		Target       string `json:"target" yaml:"target"`
		Table        string `json:"table"         yaml:"table"`
		RowsAffected int64  `json:"rows_affected" yaml:"rows_affected"`
	}

	return writeJSON(w.out, w.pr, insertOutput{
		Target:       target.Handle,
		Table:        tbl,
		RowsAffected: rowsInserted,
	})
}
