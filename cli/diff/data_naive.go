package diff

import (
	"bytes"
	"context"
	"fmt"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/samber/lo"
	"golang.org/x/exp/slices"

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
	log := lg.FromContext(ctx).With("a", query1).With("b", query2)

	pr := ru.Writers.Printing.Clone()
	pr.EnableColor(false)

	buf1, buf2 := &bytes.Buffer{}, &bytes.Buffer{}
	w1, w2 := cfg.RecordWriterFn(buf1, pr), cfg.RecordWriterFn(buf2, pr)
	recw1, recw2 := output.NewRecordWriterAdapter(ctx, w1), output.NewRecordWriterAdapter(ctx, w2)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := libsq.ExecuteSLQ(gCtx, qc, query1, recw1); err != nil {
			if errz.IsErrNotExist(err) {
				// It's totally ok if a table is not found.
				log.Debug("Diff: table not found", lga.Src, td1.src, lga.Table, td1.tblName)
				return nil
			}
		}

		_, err := recw1.Wait()
		return err
	})
	g.Go(func() error {
		if err := libsq.ExecuteSLQ(gCtx, qc, query2, recw2); err != nil {
			if errz.IsErrNotExist(err) {
				log.Debug("Diff: table not found", lga.Src, td2.src, lga.Table, td2.tblName)
				return nil
			}
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

// execSourceDataDiff executes a diff all tables found in either source.
func execSourceDataDiff(ctx context.Context, ru *run.Run, cfg *Config, sd1, sd2 *sourceData) error {
	allTblNames := append(sd1.srcMeta.TableNames(), sd2.srcMeta.TableNames()...)
	allTblNames = lo.Uniq(allTblNames)
	slices.Sort(allTblNames)

	tblDataDiffs := make([]*tableDataDiff, 0, len(allTblNames))
	for _, tbl := range allTblNames {
		select {
		case <-ctx.Done():
			return errz.Err(ctx.Err())
		default:
		}
		td1 := &tableData{
			tblName: tbl,
			src:     sd1.src,
			srcMeta: sd1.srcMeta,
		}
		td2 := &tableData{
			tblName: tbl,
			src:     sd2.src,
			srcMeta: sd2.srcMeta,
		}

		tblDataDiff, err := buildTableDataDiff(ctx, ru, cfg, td1, td2)
		if err != nil {
			return err
		}

		if tblDataDiff != nil {
			tblDataDiffs = append(tblDataDiffs, tblDataDiff)
		}
	}

	slices.SortFunc(tblDataDiffs, func(a, b *tableDataDiff) bool {
		return a.td1.tblName < b.td1.tblName
	})

	for _, tblDataDiff := range tblDataDiffs {
		select {
		case <-ctx.Done():
			return errz.Err(ctx.Err())
		default:
		}

		if err := Print(ru.Out, ru.Writers.Printing, tblDataDiff.header, tblDataDiff.diff); err != nil {
			return err
		}
	}

	return nil
}
