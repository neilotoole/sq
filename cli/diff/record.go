package diff

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/libdiff"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tailbuf"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/driver"
)

// execTableDataDiffDoc compares the row data in td1 and td2, writing the diff
// to doc. The doc is sealed via HunkDoc.Seal before the function returns. If an
// error occurs, the error is sealed in the doc, and can be checked via
// [HunkDoc.Err]. Note that the returned doc's Doc.Read method blocks until the
// doc is completed (or errors). Thus it's possible to execute this function on
// a goroutine, and then invoke Doc.Read on another goroutine.
func execTableDataDiffDoc(ctx context.Context, ru *run.Run, cfg *Config,
	td1, td2 *tableData, doc *HunkDoc,
) {
	log := lg.FromContext(ctx)
	recBufSize := tuning.OptRecBufSize.Get(options.FromContext(ctx))
	recPairs := make(chan record.Pair, recBufSize)

	qc := run.NewQueryContext(ru, nil)
	query1 := td1.src.Handle + "." + stringz.DoubleQuote(td1.tblName)
	query2 := td2.src.Handle + "." + stringz.DoubleQuote(td2.tblName)

	errCh := make(chan error, 5) // Not sure if 5 is the correct size?
	recw1 := &recordWriter{
		recCh: make(chan record.Record, recBufSize),
		errCh: errCh,
	}
	recw2 := &recordWriter{
		recCh: make(chan record.Record, recBufSize),
		errCh: errCh,
	}

	msg := fmt.Sprintf("Diff table data %s, %s", td1.String(), td2.String())
	bar := progress.FromContext(ctx).NewWaiter(msg, true, progress.OptMemUsage)

	// Well, this is a bit wonky and probably can be restructured to be simpler.
	// FIXME: more docs.
	// The idea is that we want to cancel the context if an error occurs on any
	// of the goroutines. We also want to cancel the context if the user
	// cancels the operation. We also want to cancel the context if the
	// operation completes successfully. So, we create a new context, and

	var cancelFn context.CancelCauseFunc
	ctx, cancelFn = context.WithCancelCause(ctx)
	go func() {
		if err := <-errCh; err != nil {
			cancelFn(err)
		}
	}()

	go func() {
		// Query DB, send records to recw1.
		if err := libsq.ExecuteSLQ(ctx, qc, query1, recw1); err != nil {
			if errz.Has[*driver.NotExistError](err) {
				// For diffing, it's totally ok if a table is not found.
				log.Debug("Diff: table not found", lga.Table, td1.String())
				return
			}
			cancelFn(err)
		}
	}()

	go func() {
		// Query DB, send records to recw2.
		// errCh <- errz.New("oh no") // FIXME: delete
		// return

		if err := libsq.ExecuteSLQ(ctx, qc, query2, recw2); err != nil {
			if errz.Has[*driver.NotExistError](err) {
				log.Debug("Diff: table not found", lga.Table, td2.String())
				return
			}
			cancelFn(err)
		}
	}()

	df := &recordDiffer{
		cfg: cfg,
		td1: td1,
		td2: td2,
		recMetaFn: func() (meta1, meta2 record.Meta) {
			return recw1.recMeta, recw2.recMeta
		},
	}

	go func() {
		// Consume records from recw1 and recw2, build a record.Pair,
		// and send the pair to df.recPairs. The pairs will be consumed
		// by the exec goroutine.
		var rec1, rec2 record.Record

		for i := 0; ctx.Err() == nil; i++ {
			select {
			case <-ctx.Done():
				return
			case rec1 = <-recw1.recCh:
			}

			select {
			case <-ctx.Done():
				return
			case rec2 = <-recw2.recCh:
			}

			if rec1 == nil && rec2 == nil {
				// End of data
				close(recPairs)
				return
			}

			recPairs <- record.NewPair(i, rec1, rec2)
		}
	}()

	go func() {
		// This goroutine is the main action. It calls recordDiffer.exec, which
		// consumes the record pairs, and writes the diff to doc. At the end of
		// this function, doc.Seal is invoked. There are three possibilities:
		//
		//  - Happy path: everything worked, and doc.Seal(nil) is invoked.
		//  - recordDiff.exec encountered an error, and doc.Seal(err) is invoked.
		//  - One of the other goroutines encountered an error, and propagated that
		//    error via cancelFn(err). Thus, in this goroutine, we must check that
		//    condition, and invoke doc.Seal() with the cancel cause error.

		defer bar.Stop()
		var err error
		defer func() {
			// On the happy path, err is nil, but we still want to invoke cancelFn
			// to avoid resource leaks.
			cancelFn(err)
		}()

		if err = df.exec(ctx, recPairs, doc); err != nil {
			doc.Seal(err)
			return
		}

		// On the happy path, the error arg to doc.Seal is nil.
		//
		// But, if any of the other goroutines encountered an error, that error
		// was propagated to the context via the context.CancelCauseFunc.
		doc.Seal(errz.Err(context.Cause(ctx)))
	}()
}

// recordDiffer encapsulates execution of diffing the records of two tables.
type recordDiffer struct {
	cfg      *Config
	td1, td2 *tableData

	// recMetaFn returns the record.Meta for the query results for td1 and td2.
	// We use a function here because record.Meta is only available after the
	// query has been executed (and isn't available at the time of recordDiffer
	// construction).
	recMetaFn func() (rm1, rm2 record.Meta)
}

// exec compares the record pairs from recPairs, writing the diff results to
// doc. This function does not invoke [HunkDoc.Seal], so the caller must do so,
// probably passing the returned err (if non-nil) to [HunkDoc.Seal].
func (rd *recordDiffer) exec(ctx context.Context, recPairs <-chan record.Pair, doc *HunkDoc) error {
	var (
		numLines  = rd.cfg.Lines
		tb        = tailbuf.New[record.Pair](numLines + 1)
		hunkPairs []record.Pair

		rp  record.Pair
		ok  bool
		err error
	)

LOOP:
	for row := 0; ctx.Err() == nil; row++ {
		select {
		case <-ctx.Done():
			err = errz.Err(context.Cause(ctx))
			break LOOP
		case rp, ok = <-recPairs:
		}

		if !ok {
			// We've reached the end of the record pairs.
			break
		}

		// We've got a record pair. We add it to the tailbuf.
		tb.Write(rp)

		if rp.Equal() {
			// The record pair is equal, so we loop.
			continue
		}

		// We've found a differing record pair. We need to generate a hunk.
		var hunk *Hunk

		// But, the hunk doesn't just contain the differing record pair. It may also
		// include context lines before and after the difference.

		// First, we get the before-the-difference record pairs from the tailbuf.
		// Conveniently, the tailbuf already contains the differing record pair.
		hunkPairs = tb.Slice(row-numLines, row+1)

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
		// The maxHunRecords limit exists to prevent unbounded growth of the hunk,
		// which could eventually lead to an OOM situation if the diff is huge. If
		// the limit is reached, the user will see adjacent hunks without any
		// non-differing context lines between them. But that's ok, it's still a
		// well-formed and valid diff, it'll be rare, and it's better than OOMing.
		var pairMatchSeq int
		for {
			select {
			case <-ctx.Done():
				err = errz.Err(ctx.Err())
				break LOOP
			case rp, ok = <-recPairs:
			}

			if !ok {
				// We've reached the end of the record pairs.
				break
			}

			row++
			tb.Write(rp)
			hunkPairs = append(hunkPairs, rp)

			if rp.Equal() {
				// Yay, we've found another matching record pair for our sequence.
				pairMatchSeq++
			} else {
				// Boo, the contiguous matching sequence has been broken.
				pairMatchSeq = 0
			}

			if len(hunkPairs) >= rd.cfg.HunkMaxSize {
				// We've reached the hard limit for the hunk size.
				break
			}

			if pairMatchSeq > numLines*2 {
				// We've looked ahead far enough to avoid the adjacent hunk line
				// duplication issue, so we can trim off those extra lookahead pairs.
				hunkPairs = hunkPairs[:len(hunkPairs)-numLines]
				break
			}
		}

		// OK, now we've got enough record pairs to populate the hunk.

		rd.populateHunk(ctx, hunkPairs, hunk)
		if err = hunk.Err(); err != nil {
			break
		}
	}

	if err == nil {
		// Even if err is nil, it's still possible that ctx.Err is non-nil.
		err = errz.Err(context.Cause(ctx))
	}

	return err
}

// populateHunk populates hunk with the diff of the record pairs. Before return,
// the hunk is always sealed via [Hunk.Seal]. The caller can check [Hunk.Err]
// to see if an error occurred.
func (rd *recordDiffer) populateHunk(ctx context.Context, pairs []record.Pair, hunk *Hunk) {
	var (
		handleTbl1           = rd.td1.String()
		handleTbl2           = rd.td2.String()
		recMeta1, recMeta2   = rd.recMetaFn()
		hunkHeader, hunkBody string
		body1, body2         string
		err                  error
	)

	defer func() {
		// We always seal the hunk. Note that hunkHeader is populated at the bottom
		// of the function. But if an error occurs and the function is returning
		// early, it's ok if hunkHeader is empty.
		hunk.Seal([]byte(hunkHeader), err)
	}()

	recs1 := make([]record.Record, len(pairs))
	recs2 := make([]record.Record, len(pairs))
	for i := range pairs {
		recs1[i] = pairs[i].Rec1()
		recs2[i] = pairs[i].Rec2()
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var bodyErr error
		body1, bodyErr = renderRecords(gCtx, rd.cfg, recMeta1, recs1)
		return bodyErr
	})
	g.Go(func() error {
		var bodyErr error
		body2, bodyErr = renderRecords(gCtx, rd.cfg, recMeta2, recs2)
		return bodyErr
	})
	if err = g.Wait(); err != nil {
		return
	}

	var unified string
	if unified, err = libdiff.ComputeUnified(
		ctx,
		handleTbl1,
		handleTbl2,
		rd.cfg.Lines,
		body1,
		body2,
	); err != nil {
		return
	}

	// Trim the diff "file header"... ultimately, we should change ComputeUnified
	// to not return this (e.g. add an arg "noHeader=true")
	unified = stringz.TrimHead(unified, 2)

	var ok bool
	if hunkHeader, hunkBody, ok = strings.Cut(unified, "\n"); !ok {
		err = errz.New("Hunk header not found")
		return
	}

	if err = colorizeHunks(ctx, hunk, rd.cfg.prDiff, bytes.NewReader(stringz.UnsafeBytes(hunkBody))); err != nil {
		return
	}

	if hunkHeader, err = adjustHunkOffset(hunkHeader, hunk.offset); err != nil {
		return
	}

	// hunkHeader will be passed to hunk.Seal in the top defer.
	hunkHeader = rd.cfg.prDiff.Section.Sprintln(hunkHeader)
}

var _ libsq.RecordWriter = (*recordWriter)(nil)

// recordWriter is a trivial [libsq.RecordWriter] impl, whose recCh field is
// used to capture records returned from a query.
type recordWriter struct {
	recCh   chan record.Record
	errCh   chan error
	recMeta record.Meta
}

// Open implements libsq.RecordWriter.
func (w *recordWriter) Open(_ context.Context, _ context.CancelFunc, recMeta record.Meta,
) (recCh chan<- record.Record, errCh <-chan error, err error) {
	w.recMeta = recMeta
	return w.recCh, w.errCh, nil
}

// Wait implements libsq.RecordWriter. It won't ever be invoked, so it's no-op
// and returns zero values.
func (w *recordWriter) Wait() (written int64, err error) {
	// We don't actually use Wait(), so just return zero values.
	return 0, nil
}

// adjustHunkOffset adjusts the offset of a diff hunk. The hunk input is
// expected to be a string of one of two forms. This is the long form:
//
//	@@ -44,7 +44,7 @@
//
// Given an offset of 10, this would become:
//
//	@@ -54,7 +54,7 @@
//
// The short form is:
//
//	@@ -44 +44 @@
//
// Given an offset of 10, this would become:
//
//	@@ -54 +54 @@
//
// The short form is used when there's no surrounding lines (-U=0).
//
// Note that "-44,7 +44,7" means that the first line shown is line 44 and the
// number of lines compared is 7 (although 8 lines will be rendered, because the
// changed line is shown twice: the before and after versions of the line).
func adjustHunkOffset(hunk string, offset int) (string, error) {
	// https://unix.stackexchange.com/questions/81998/understanding-of-diff-output
	const formatShort = "@@ -%d +%d @@"
	const formatFull = "@@ -%d,%d +%d,%d @@"

	var i1, i2, i3, i4 int
	count, err := fmt.Fscanf(strings.NewReader(hunk), formatFull, &i1, &i2, &i3, &i4)
	if err == nil {
		if count != 4 {
			return "", errz.Errorf("expected 4 values, got %d", count)
		}

		i1 += offset
		i3 += offset

		return fmt.Sprintf(formatFull, i1, i2, i3, i4), nil
	}

	// Long format didn't work, try the short format.
	_, err = fmt.Fscanf(strings.NewReader(hunk), formatShort, &i1, &i3)
	if err != nil {
		return "", errz.Errorf("failed to parse Hunk: %s", hunk)
	}

	i1 += offset
	i3 += offset

	return fmt.Sprintf(formatShort, i1, i3), nil
}
