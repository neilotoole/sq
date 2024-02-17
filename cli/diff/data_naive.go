package diff

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/libdiff"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/driver"
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
// See: https://github.com/neilotoole/sq/issues/353.
func buildTableDataDiff(ctx context.Context, ru *run.Run, cfg *Config, //nolint:unused
	td1, td2 *tableData,
) (*tableDataDiff, error) {
	qc := run.NewQueryContext(ru, nil)
	query1 := td1.src.Handle + "." + td1.tblName
	query2 := td2.src.Handle + "." + td2.tblName
	log := lg.FromContext(ctx).With("a", query1).With("b", query2)

	pr := ru.Writers.OutPrinting.Clone()
	pr.EnableColor(false)

	buf1, buf2 := &bytes.Buffer{}, &bytes.Buffer{}
	w1, w2 := cfg.RecordWriterFn(buf1, pr), cfg.RecordWriterFn(buf2, pr)
	recw1, recw2 := output.NewRecordWriterAdapter(ctx, w1), output.NewRecordWriterAdapter(ctx, w2)

	bar := progress.FromContext(ctx).NewWaiter("Retrieving diff data", true, progress.OptMemUsage)
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := libsq.ExecuteSLQ(gCtx, qc, query1, recw1); err != nil {
			if errz.Has[*driver.NotExistError](err) {
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
			if errz.Has[*driver.NotExistError](err) {
				log.Debug("Diff: table not found", lga.Src, td2.src, lga.Table, td2.tblName)
				return nil
			}
		}

		_, err := recw2.Wait()
		return err
	})
	err := g.Wait()
	bar.Stop()
	if err != nil {
		return nil, err
	}

	body1, body2 := buf1.String(), buf2.String()

	bar2 := progress.FromContext(ctx).NewWaiter("Diff table "+td1.String(), true, progress.OptMemUsage)
	unified, err := libdiff.ComputeUnified(ctx, query1, query2, cfg.Lines, body1, body2)
	bar2.Stop()
	if err != nil {
		return nil, err
	}

	return &tableDataDiff{
		td1:    td1,
		td2:    td2,
		header: fmt.Sprintf("sq diff --data %s %s", query1, query2),
		diff:   unified,
	}, nil
}

func execSourceDataDiff(ctx context.Context, ru *run.Run, cfg *Config, sd1, sd2 *sourceData) (err error) {
	log := lg.FromContext(ctx).With(lga.Left, sd1.src.Handle, lga.Right, sd2.src.Handle)
	log.Info("Diffing source data")

	cfg.prMain = ru.Writers.OutPrinting.Clone() // FIXME: This should happen automatically in NewConfig
	cfg.prDiff = ru.Writers.OutPrinting.Diff.Clone()

	allTblNames := append(sd1.srcMeta.TableNames(), sd2.srcMeta.TableNames()...)
	allTblNames = lo.Uniq(allTblNames)
	slices.Sort(allTblNames)

	var cancelFn context.CancelCauseFunc
	ctx, cancelFn = context.WithCancelCause(ctx)
	defer func() {
		cancelFn(err)
	}()

	g := &errgroup.Group{}
	g.SetLimit(tuning.OptErrgroupLimit.Get(options.FromContext(ctx)))

	docs := make([]*HunkDoc, len(allTblNames))
	execFns := make([]func() error, len(allTblNames))
	for i, tblName := range allTblNames {
		td1 := &tableData{src: sd1.src, tblName: tblName}
		td1.tblMeta = sd1.srcMeta.Table(tblName)

		// REVISIT: What if there isn't table metadata? Should we fetch
		// it on a goroutine?

		td2 := &tableData{src: sd2.src, tblName: tblName}
		td2.tblMeta = sd2.srcMeta.Table(tblName)

		cmd := cfg.prDiff.Command.Sprintf("sq diff --data %s %s", td1.String(), td2.String())
		doc := NewHunkDoc(cmd, NewDocHeader(cfg.prDiff, td1.String(), td2.String()))
		docs[i] = doc
		execFns[i] = func() error {
			execTableDataDiffDoc(ctx, cancelFn, ru, cfg, td1, td2, doc)
			return doc.Err()
		}
	}

	for i := range execFns {
		g.Go(execFns[i])
	}

	if err = g.Wait(); err != nil {
		log.Error("Goroutine error", lga.Err, err)
		cancelFn(err)
		return err
	}

	for i := range docs {
		doc := docs[i]
		if err = doc.Err(); err != nil {
			log.Error("Diff doc error")
			cancelFn(err)
			return err
		}

		if _, err = io.Copy(ru.Out, contextio.NewReader(ctx, doc)); err != nil {
			log.Error("Doc write out error", lga.Err, err)
			cancelFn(err)
			return err
		}
	}

	return nil
}

// execSourceDataDiff executes a diff of all tables found in either source.
func execSourceDataDiffOld(ctx context.Context, ru *run.Run, cfg *Config, sd1, sd2 *sourceData) error { //nolint:unused
	o := options.FromContext(ctx)

	allTblNames := append(sd1.srcMeta.TableNames(), sd2.srcMeta.TableNames()...)
	allTblNames = lo.Uniq(allTblNames)
	slices.Sort(allTblNames)

	diffs := make([]*tableDataDiff, len(allTblNames))

	// mIndex is a map of table name to its index in allTblNames.
	mIndex := make(map[string]int, len(allTblNames))
	for i := range allTblNames {
		mIndex[allTblNames[i]] = i
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(tuning.OptErrgroupLimit.Get(o))
	diffCh := make(chan *tableDataDiff, tuning.OptRecBufSize.Get(o))
	// REVISIT: Why are we using tuning.OptRecBufSize? It's not a record buffer.

	printErrCh := make(chan error, 1)
	printIndex := 0
	go func() {
		// This is the read/print goroutine. Further down, in an errgroup, we spin
		// up multiple goroutines to build a tableDataDiff for each table. Those
		// errgroup goroutines write their results to diffCh. But they can be sent
		// on diffCh in any order.
		//
		// This goroutine reads *tblDataDiff from diffCh, and looks up the index
		// of that table in mIndex. Then, it sets diffs[i] with the tableDataDiff.
		//
		// Note that we want to print the diffs in the order specified by
		// allTblNames. A printIndex counter is maintained. After the goroutine
		// inserts a value into diffs, we then check if the next (as defined by
		// printIndex) element of diffs is non-nil. If so, the goroutine prints
		// that diff, and advances the printIndex counter.

		defer func() { close(printErrCh) }()

		var tblDataDiff *tableDataDiff
		for {
			select {
			case <-gCtx.Done():
				return
			case tblDataDiff = <-diffCh:
				if tblDataDiff == nil {
					// Channel is closed, means we're done.
					return
				}
			}

			diffIndex, ok := mIndex[tblDataDiff.td1.tblName]
			if !ok {
				// Shouldn't happen
				err := errz.Errorf("Index not found for table: %s", tblDataDiff.td1.tblName)
				printErrCh <- err
				return
			}

			// Put tblDataDiff into diffs.
			diffs[diffIndex] = tblDataDiff

			// Now check if the next diff is available to print.
			for {
				select {
				case <-gCtx.Done():
					return
				default:
				}

				if printIndex >= len(diffs) {
					return
				}

				if diffs[printIndex] == nil {
					break
				}

				tblDataDiff = diffs[printIndex]
				if err := Print(
					ctx,
					ru.Out,
					ru.Writers.OutPrinting.Diff,
					tblDataDiff.header,
					strings.NewReader(tblDataDiff.diff),
				); err != nil {
					printErrCh <- err
					return
				}

				printIndex++
			}
		}
	}()

	for _, tblName := range allTblNames {
		tblName := tblName

		// Append a little delay to allow the goroutine to get a head start
		// on its successors. Benchmark: does this actually help?
		time.Sleep(time.Microsecond * 10)

		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return errz.Err(gCtx.Err())
			default:
			}

			td1 := &tableData{
				tblName: tblName,
				src:     sd1.src,
				srcMeta: sd1.srcMeta,
			}
			td2 := &tableData{
				tblName: tblName,
				src:     sd2.src,
				srcMeta: sd2.srcMeta,
			}

			tblDataDiff, err := buildTableDataDiff(gCtx, ru, cfg, td1, td2)
			if err != nil {
				return err
			}

			diffCh <- tblDataDiff
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return err
	}
	close(diffCh)

	// This has the effect of waiting until the print goroutine completes.
	err := <-printErrCh
	return err
}
