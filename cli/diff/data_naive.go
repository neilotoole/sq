package diff

import (
	"bytes"
	"context"
	"fmt"

	"github.com/neilotoole/sq/cli/diff/internal/go-udiff"
	"github.com/neilotoole/sq/cli/diff/internal/go-udiff/myers"

	"github.com/neilotoole/sq/cli/output"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// buildTableDataDiff compares the row data in td1 and td2, returning
// a tableDataDiff instance if there's a difference between the
// equivalent rows.
//
// NOTE: This is a naive implementation. It simply prints the entire
// results from each table to an output.RecordWriter, and diffs the
// resulting text. Thus, if the target tables contained a million rows
// but no diffs, each row will still have been rendered to text.
// A better mechanism might be to perform the diff against the
// raw record.Record values, and only generate the diff text if there
// are differences, and even then, to only selectively generate the
// needed text.
func buildTableDataDiff(ctx context.Context, ru *run.Run, cfg *Config,
	td1, td2 *tableData,
) (*tableDataDiff, error) {
	qc := run.NewQueryContext(ru)
	query1 := td1.src.Handle + "." + td1.tblName
	query2 := td2.src.Handle + "." + td2.tblName

	pr := ru.Writers.Printing.Clone()
	pr.EnableColor(false)

	buf1, buf2 := &bytes.Buffer{}, &bytes.Buffer{}
	w1, w2 := cfg.RecordWriterFn(buf1, pr), cfg.RecordWriterFn(buf2, pr)
	recw1, recw2 := output.NewRecordWriterAdapter(ctx, w1), output.NewRecordWriterAdapter(ctx, w2)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := libsq.ExecuteSLQ(gCtx, qc, query1, recw1); err != nil {
			return err
		}

		_, err := recw1.Wait()
		return err
	})
	g.Go(func() error {
		if err := libsq.ExecuteSLQ(gCtx, qc, query2, recw2); err != nil {
			return err
		}

		_, err := recw2.Wait()
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	var (
		body1, body2 = buf1.String(), buf2.String()
		err          error
	)

	edits := myers.ComputeEdits(body1, body2)
	unified, err := udiff.ToUnified(
		query1,
		query2,
		body1,
		edits,
		cfg.Lines,
	)
	if err != nil {
		return nil, errz.Err(err)
	}

	return &tableDataDiff{
		td1:    td1,
		td2:    td2,
		header: fmt.Sprintf("sq diff --data %s %s", query1, query2),
		diff:   unified,
	}, nil
}
