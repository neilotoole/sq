package diff

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tailbuf"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// differsForAllTableData returns a slice containing a *diffdoc.Differ for the
// row data of each table in src1 and src2.
func differsForAllTableData(ctx context.Context, cfg *Config, src1, src2 *source.Source,
) (differs []*diffdoc.Differ, err error) {
	log := lg.FromContext(ctx).With(lga.Left, src1.Handle, lga.Right, src2.Handle)
	log.Info("Diffing source tables data")

	tbls1, tbls2, err := cfg.Run.MDCache.TableNamesPair(ctx, src1, src2)
	if err != nil {
		return nil, err
	}

	allTblNames := lo.Uniq(langz.JoinSlices(tbls1, tbls2))
	slices.Sort(allTblNames)

	differs = make([]*diffdoc.Differ, len(allTblNames))
	for i, tblName := range allTblNames {
		td1 := source.Table{Handle: src1.Handle, Name: tblName}
		td2 := source.Table{Handle: src2.Handle, Name: tblName}
		differs[i] = differForTableData(cfg, true, td1, td2)
	}

	return differs, nil
}

func differForTableData(cfg *Config, title bool, td1, td2 source.Table) *diffdoc.Differ {
	var cmdTitle diffdoc.Title
	if title {
		if cfg.StopAfter > 0 {
			cmdTitle = diffdoc.Titlef(cfg.Colors, "sq diff --data --stop %d %s %s", cfg.StopAfter, td1, td2)
		} else {
			cmdTitle = diffdoc.Titlef(cfg.Colors, "sq diff --data %s %s", td1, td2)
		}
	}

	doc := diffdoc.NewHunkDoc(
		cmdTitle,
		diffdoc.Headerf(cfg.Colors, td1.String(), td2.String()))
	differ := diffdoc.NewDiffer(doc, func(ctx context.Context, cancelFn func(error)) {
		diffTableData(ctx, cancelFn, cfg, td1, td2, doc)
		if doc.Err() != nil {
			cancelFn(doc.Err())
		}
	})

	return differ
}

// diffTableData compares the row data in td1 and td2, writing the diff to doc.
// The doc is sealed via [diffdoc.HunkDoc.Seal] before the function returns. If
// an error occurs, the error is sealed into the doc, and can be checked via
// [diffdoc.HunkDoc.Err]. Any error should also be propagated via cancelFn, to
// cancel any peer goroutines. Note that the returned doc's [diffdoc.Doc.Read]
// method blocks until the doc is completed (or errors out).
func diffTableData(ctx context.Context, cancelFn context.CancelCauseFunc, //nolint:funlen,gocognit
	cfg *Config, td1, td2 source.Table, doc *diffdoc.HunkDoc,
) {
	log := lg.FromContext(ctx).With(lga.Left, td1.String(), lga.Right, td2.String())
	log.Info("Diffing table data")

	bar := progress.FromContext(ctx).NewWaiter(
		fmt.Sprintf("Diff table data %s, %s", td1.String(), td2.String()),
		progress.OptMemUsage,
	)

	recBufSize := tuning.OptRecBufSize.Get(options.FromContext(ctx))
	recPairsCh := make(chan record.Pair, recBufSize)

	// We create two dbResults instances (that implement libsq.RecordWriter), each
	// of which will capture the records returned from a query. On a separate
	// goroutine, those records will be collated into record.Pair instances, and
	// sent to recPairsCh. Then, those record pairs are used to generate the diff,
	// which is written to doc.
	//
	// Note that the libsq.RecordWriter.Open method must return an error channel
	// that the query engine can send errors to. That error channel is errCh,
	// created directly below.
	errCh := make(chan error, 5) // Not sure if 5 is the correct size?

	// The two dbResults instances (rs1 and rs2) share the same errCh, because we
	// don't care which one receives an error, just that one of them did.

	rs1 := &dbResults{
		recCh: make(chan record.Record, recBufSize),
		errCh: errCh,
	}
	rs2 := &dbResults{
		recCh: make(chan record.Record, recBufSize),
		errCh: errCh,
	}

	// Somebody has to listen for errors on errCh. If an error is received, we'll
	// cancel ctx, which will stop the other goroutines.
	go func() {
		if err := <-errCh; err != nil {
			switch {
			case errz.Has[*driver.NotExistError](err):
				// For diffing, it's totally ok if a table is not found.
				log.Warn("Diff: table not found")
				return
			case errors.Is(err, context.Canceled):
				log.Warn("Diff: cancelled err on errCh consumer")
				return
			case errors.Is(err, errz.ErrStop):
				// errz.ErrStop is normal control flow (handled elsewhere), not an
				// error, so we don't log anything. I'm not sure if this case is even
				// reachable, but it's here just in case.
				return
			default:
				log.Error("Error from record writer errCh", lga.Err, err)
				cancelFn(err)
			}
		}
	}()

	// Now we'll start two goroutines to execute the DB queries. The resulting
	// records from the DB queries will be sent to each dbResults.recCh, and
	// any errors will be sent to the shared errCh.

	qc := run.NewQueryContext(cfg.Run, nil)

	// We give the DB query goroutines their own context, dbCtx. This is so that
	// we can explicitly stop the queries using dbCancel(errz.ErrStop) if we reach
	// the diff stop-after limit.
	dbCtx, dbCancel := context.WithCancelCause(ctx)
	go func() {
		query1 := td1.Handle + "." + stringz.DoubleQuote(td1.Name)
		// Execute DB query1; records will be sent to rs1.recCh.
		if err := libsq.ExecuteSLQ(dbCtx, qc, query1, rs1); err != nil {
			switch {
			case errz.Has[*driver.NotExistError](err):
				// For diffing, it's totally ok if a table is not found.
				log.Debug("Diff: table not found", lga.Table, td2.String())
				return
			case errors.Is(err, errz.ErrStop) || errz.IsContextStop(dbCtx):
				// This means we explicitly stopped the query, probably due to reaching
				// the diff stop-after limit.
				return
			}

			// Bah! An error was returned from libsq.ExecuteSLQ. This error may have
			// arisen even before the query was executed, and thus is not guaranteed
			// to have been sent on errCh. Regardless, we cancel the context with
			// the error (although it's highly likely it's already been canceled).
			cancelFn(err)
			if !errz.IsErrContext(err) {
				// No need to generate logs for context errors; the cause will be
				// logged elsewhere.
				log.Error("Error executing query", lga.Table, td1.String(), lga.Err, err)
			}
		}
	}()

	go func() {
		query2 := td2.Handle + "." + stringz.DoubleQuote(td2.Name)
		// Execute DB query2; records will be sent to rs2.recCh.
		if err := libsq.ExecuteSLQ(dbCtx, qc, query2, rs2); err != nil {
			switch {
			case errz.Has[*driver.NotExistError](err):
				// For diffing, it's totally ok if a table is not found.
				log.Debug("Diff: table not found", lga.Table, td2.String())
				return
			case errors.Is(err, errz.ErrStop) || errz.IsContextStop(dbCtx):
				// This means we explicitly stopped the query, probably due to
				// reaching the diff stop-after limit.
				return
			default:
			}

			cancelFn(err)
			if !errz.IsErrContext(err) {
				// No need to generate logs for context errors; the cause will be
				// logged elsewhere.
				log.Error("Error executing query", lga.Table, td1.String(), lga.Err, err)
			}
		}
	}()

	// At this point, both of our DB query goroutines have kicked off. This next
	// goroutine collates the records from rs1 and rs2 into record.Pair instances,
	// and sends those pairs to recPairsCh. The pairs will be consumed by the diff
	// exec goroutine further below.
	go func() {
		var rec1, rec2 record.Record
		var diffCount int

		// If cfg.StopAfter is set, we'll stop after diffCount reaches cfg.StopAfter
		// plus cfg.Lines. We add cfg.Lines to ensure that we have enough records
		// to generate the "context lines" after the last differing record.
		stopAt := -1

		for i := 0; ctx.Err() == nil; i++ {
			select {
			case <-ctx.Done():
				return
			case rec1 = <-rs1.recCh:
			}

			select {
			case <-ctx.Done():
				return
			case rec2 = <-rs2.recCh:
			}

			if rec1 == nil && rec2 == nil {
				// End of data
				close(recPairsCh)
				return
			}

			rp := record.NewPair(i, rec1, rec2)
			if !rp.Equal() {
				diffCount++
			}

			recPairsCh <- rp

			if stopAt == -1 && cfg.StopAfter > 0 && diffCount >= cfg.StopAfter {
				stopAt = i + cfg.Lines
			}
			if stopAt > -1 && i >= stopAt {
				dbCancel(errz.ErrStop) // Explicit stop
				close(recPairsCh)
				return
			}
		}
	}()

	// This final goroutine is the main action.
	go func() {
		defer bar.Stop() // Now is as good a time as any to cancel the progress bar.

		// First, we construct a recordDiffer instance. It encapsulates building
		// the diff from the record pairs in recPairsCh.
		recDiffer := &recordDiffer{
			cfg: cfg,
			td1: td1,
			td2: td2,
			recMetaFn: func() (meta1, meta2 record.Meta) {
				return rs1.recMeta, rs2.recMeta
			},
		}

		// Shortly below, we invoke recordDiffer.exec, which consumes the record
		// pairs from recPairsCh, and writes the diff to doc. At the end of this
		// unction, doc.Seal is invoked. There are three possibilities:
		//
		//  - Happy path: everything worked, and doc.Seal(nil) is invoked.
		//  - recordDiffer.exec encountered an error, and doc.Seal(err) is invoked.
		//  - One of the other goroutines encountered an error, and propagated that
		//    error via cancelFn(err). Thus, in this goroutine, we must check that
		//    condition, and invoke doc.Seal() with the cancel cause error.

		var err error

		// OK, finally we get to generating the diff! The generated diff is written
		// to doc.
		if err = recDiffer.exec(ctx, recPairsCh, doc); err != nil {
			// Something bad happened, err is non-nil. Propagate err to the doc, and
			// get the hell outta here.
			if !errz.IsErrContext(err) {
				// No need to generate logs for context errors; the cause will be
				// logged elsewhere.
				log.Error("Error generating diff", lga.Err, err)
			}

			doc.Seal(err)
			return
		}

		// We didn't get an error from recordDiffer.exec. Presumably we're on the
		// happy path, and so the error arg to doc.Seal should be nil.
		//
		// BUT... if any of the other goroutines encountered an error, that error
		// was propagated to ctx via cancelFn, and we would need to pass that error
		// to doc.Seal.
		//
		// But hopefully we're just passing nil to doc.Seal here.
		err = errz.Err(context.Cause(ctx))
		if err != nil {
			log.Error("Record differ: post-execution: error in ctx", lga.Err, err)
		}
		doc.Seal(err)
	}()

	// Now diffTableData returns, while the goroutines do their magic.
	//
	// The caller can just invoke doc.Read, which will block until the DB queries
	// execute and return results, and the diff is generated, and the diff is
	// written to doc.
}

// recordDiffer encapsulates execution of diffing the records of two tables.
type recordDiffer struct {
	cfg *Config

	// recMetaFn returns the record.Meta for the query results for td1 and td2.
	// We use a function here because record.Meta is only available after the
	// query has been executed (the record.Meta is returned from the DB, and thus
	// isn't guaranteed to be available at the time of recordDiffer construction).
	recMetaFn func() (rm1, rm2 record.Meta)
	td1, td2  source.Table
}

// exec compares the record pairs from recPairsCh, writing the diff results to
// doc. This function does not invoke [HunkDoc.Seal], so the caller must do so,
// probably passing the returned err (if non-nil) to [HunkDoc.Seal].
func (rd *recordDiffer) exec(ctx context.Context, recPairsCh <-chan record.Pair, doc *diffdoc.HunkDoc) error {
	var (
		numLines = rd.cfg.Lines

		// We use a tailbuf to hang on to the last X record pairs. We'll need to
		// look back at those record pairs to construct the context lines preceding
		// any differing records we encounter.
		tb = tailbuf.New[record.Pair](numLines + 1)

		// hunkPairs is the slice of record pairs that will be used to generate the
		// actual diff hunk. It will contain the differing record pair, as well as
		// numLines of pairs before and after differing pair.
		hunkPairs []record.Pair

		rp      record.Pair
		ok      bool
		err     error
		ctxDone = ctx.Done()
	)

	var hunkPairsLen int
	_ = hunkPairsLen

LOOP:
	for row := 0; ctx.Err() == nil; row++ {
		select {
		case <-ctxDone:
			err = errz.Err(context.Cause(ctx))
			break LOOP
		case rp, ok = <-recPairsCh:
			// Get the next record pair for processing.
		}

		if !ok {
			// We've reached the end of the record pairs.
			break
		}

		// We've got a record pair. We add it to the tailbuf.
		tb.Write(rp)

		if rp.Equal() {
			// The record pair is equal, so we loop until we find a differing pair.
			continue
		}

		// We've found a differing record pair. We need to generate a diff hunk.
		var hunk *diffdoc.Hunk

		// But, the hunk doesn't just contain the differing record pair. It may also
		// include context lines before and after the difference.

		// First, we get the before-the-difference record pairs from the tailbuf.
		// Conveniently, the tailbuf also already contains the differing record pair.
		hunkPairs = tailbuf.SliceNominal(tb, row-numLines, row+1)
		hunkPairsLen = len(hunkPairs)

		// Create a new hunk in doc. The actual diff text will get written to that
		// hunk.
		if hunk, err = doc.NewHunk(row - (len(hunkPairs) - 1)); err != nil {
			break
		}

		// Now we need to get the after-the-difference record pairs. We look for a
		// sequence of non-differing (matching) record pairs, appending each
		// pair to hunkPairs. We stop when:
		//
		//  - we've found numLines*2 of contiguous matching record pairs, or
		//  - we've reached the end of the record pairs, or
		//  - we've accumulated cfg.HunkMaxSize pairs.
		//
		// We look ahead for numLines*2 (instead of just numLines) to avoid the
		// situation where directly adjacent hunks duplicate the last line of the
		// earlier hunk as the first line of the later hunk, e.g.
		//
		//  -34        AUDREY      OLIVIER    2020-06-11T02:50:54Z
		//  +34        AUDREY      SWIFT      2020-06-11T02:50:54Z
		//   35        JUDY        DEAN       2020-06-11T02:50:54Z
		//   36        BURT        DUKAKIS    2020-06-11T02:50:54Z
		//  @@ -36,6 +36,6 @@
		//   36        BURT        DUKAKIS    2020-06-11T02:50:54Z
		//   37        VAL         BOLGER     2020-06-11T02:50:54Z
		//  -38        TOM         MCKELLEN   2020-06-11T02:50:54Z
		//
		// The cfg.HunkMaxSize limit exists to prevent unbounded growth of the hunk,
		// which could eventually lead to an OOM situation if the diff is huge. If
		// the limit is reached, the user will see adjacent hunks without any
		// non-differing context lines between them. But that's ok, it's still a
		// well-formed and valid diff, it'll be rare, and it's better than OOMing.
		var pairMatchSeq int
		for err = ctx.Err(); err == nil; {
			// Start looking ahead to get numLines of after-the-difference record
			// pairs.

			if len(hunkPairs) >= rd.cfg.HunkMaxSize {
				// We've reached the hard limit for the hunk size.
				//front := tb.Front()
				//tb.Reset()
				//tb.Write(front)

				break
			}

			select {
			case <-ctxDone:
				err = errz.Err(context.Cause(ctx))
				break LOOP
			case rp, ok = <-recPairsCh:
			}

			if !ok {
				// We've reached the end of the record pairs.
				break
			}

			row++
			tb.Write(rp)
			hunkPairs = append(hunkPairs, rp)
			hunkPairsLen = len(hunkPairs)
			if rp.Equal() {
				// Yay, we've found another matching record pair for our sequence.
				pairMatchSeq++
			} else {
				// Boo, the contiguous matching sequence has been broken.
				pairMatchSeq = 0
			}

			if numLines == 0 {
				// Special handling for zero context lines. We need to keep looking
				// ahead until we find the first non-differing record pair, so that we
				// don't end up with adjacent hunks. If all the records differ, we'll
				// probably run into the diff.stop or diff.hunk-max-size limit at some
				// point.
				if rp.Equal() {
					// We found a non-differing pair, but because we're showing zero
					// context lines, we need to trim it off.
					hunkPairs = hunkPairs[:len(hunkPairs)-1]
					break
				}
			} else {
				if pairMatchSeq >= numLines*2 {
					// We've looked ahead far enough to avoid the adjacent hunk line
					// duplication issue, so we can trim off those extra lookahead pairs.
					hunkPairs = hunkPairs[:len(hunkPairs)-numLines]
					break
				}
			}
		}

		// OK, now we've got enough record pairs to populate the hunk.
		rd.populateHunk(ctx, hunkPairs, hunk)
		//if hunkPairsLen >= rd.cfg.HunkMaxSize {
		tb.Reset()
		//}

		if err = hunk.Err(); err != nil {
			// Uh-oh, something bad happened while populating the hunk.
			// Time to head for the exit.
			break
		}
	}

	// We're here because we either have generated the hunk, or an error occurred.
	if err == nil {
		// Even if err is nil, it's still possible that the context was canceled.
		err = errz.Err(context.Cause(ctx))
	}

	return err
}

func (rd *recordDiffer) populateHunk(ctx context.Context, pairs []record.Pair, hunk *diffdoc.Hunk) {
	rm1, rm2 := rd.recMetaFn()
	rd.cfg.RecordHunkWriter.WriteHunk(ctx, hunk, rm1, rm2, pairs)
}

var _ libsq.RecordWriter = (*dbResults)(nil)

// dbResults is a trivial [libsq.RecordWriter] impl, whose recCh field is
// used to capture records returned from a query.
type dbResults struct {
	recCh   chan record.Record
	errCh   chan error
	recMeta record.Meta
}

// Open implements libsq.RecordWriter.
func (rs *dbResults) Open(_ context.Context, _ context.CancelFunc, recMeta record.Meta,
) (recCh chan<- record.Record, errCh <-chan error, err error) {
	rs.recMeta = recMeta
	return rs.recCh, rs.errCh, nil
}

// Wait implements libsq.RecordWriter. It won't ever be invoked, so it's no-op
// and returns zero values.
func (rs *dbResults) Wait() (written int64, err error) {
	// We don't actually use Wait(), so just return zero values.
	return 0, nil
}
