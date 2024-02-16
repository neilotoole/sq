package diff

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/driver"
	"strings"

	"github.com/neilotoole/sq/libsq/core/tailbuf"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// execTableDataDiffDoc compares the row data in td1 and td2, writing the diff
// to doc.
func execTableDataDiffDoc(ctx context.Context, ru *run.Run, cfg *Config, doc *hunkDoc, td1, td2 *tableData) error {
	log := lg.FromContext(ctx)
	recBufSize := tuning.OptRecBufSize.Get(options.FromContext(ctx))

	qc := run.NewQueryContext(ru, nil)
	query1 := td1.src.Handle + "." + stringz.DoubleQuote(td1.tblName)
	query2 := td2.src.Handle + "." + stringz.DoubleQuote(td2.tblName)

	errCh := make(chan error, 5)
	recw1 := &recordWriter{
		recCh: make(chan record.Record, recBufSize),
		errCh: errCh,
	}
	recw2 := &recordWriter{
		recCh: make(chan record.Record, recBufSize),
		errCh: errCh,
	}

	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()

	// Query DB, send records to recw1.
	go func() {
		if err := libsq.ExecuteSLQ(ctx, qc, query1, recw1); err != nil {
			if errz.Has[*driver.NotExistError](err) {
				// For diffing, it's totally ok if a table is not found.
				log.Debug("Diff: table not found", lga.Table, td1.String())
				return
			}
			errCh <- err
		}
	}()
	// Query DB, send records to recw2.
	go func() {
		if err := libsq.ExecuteSLQ(ctx, qc, query2, recw2); err != nil {
			if errz.Has[*driver.NotExistError](err) {
				log.Debug("Diff: table not found", lga.Table, td2.String())
				return
			}
			errCh <- err
		}
	}()

	df := &recordDiffer{
		td1:      td1,
		td2:      td2,
		recw1:    recw1,
		recw2:    recw2,
		recPairs: make(chan record.Pair, recBufSize),
		cfg:      cfg,
		doc:      doc,
	}

	// Consume records from recw1 and recw2, build a record.Pair,
	// and send the pair to df.recPairs. The pairs will be consumed
	// by the exec goroutine.
	go func() {
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
				close(df.recPairs)
				return
			}

			df.recPairs <- record.NewPair(i, rec1, rec2)
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := df.exec(ctx); err != nil {
			errCh <- err
		}
	}()

	// Now, we wait for action. One of three things can happen...
	var err error
	select {
	case <-ctx.Done():
		// 1. The context was canceled from above.
		err = errz.Err(ctx.Err())
	case err = <-errCh:
		// 2. An error occurred in one of the goroutines.
	case <-done:
		// 3. The exec goroutine has finished, but it could be finished
		// because it's done, or because it errored. We need to check.
		select {
		case err = <-errCh:
			// ACHSCHUALLLY, the exec errored.
		default:
		}
	}

	return err
}

// recordDiffer encapsulates execution of diffing the records of two tables.
type recordDiffer struct {
	cfg          *Config
	td1, td2     *tableData
	recw1, recw2 *recordWriter
	recPairs     chan record.Pair
	doc          *hunkDoc
}

// exec compares the records in df.td1 and df.td2, writing the results
// to recordDiffer.doc.
func (rd *recordDiffer) exec(ctx context.Context) error {
	var (
		tb        = tailbuf.New[record.Pair](rd.cfg.Lines + 1)
		hunkPairs []record.Pair
		rp        record.Pair
		ok        bool
		err       error
	)

LOOP:
	for row := 0; ctx.Err() == nil; row++ {
		select {
		case <-ctx.Done():
			err = errz.Err(ctx.Err())
			break LOOP
			//return errz.Err(ctx.Err())
		case rp, ok = <-rd.recPairs:
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

		var hnk *hunk
		if hnk, err = rd.doc.newHunk(row); err != nil {
			break
		}

		// But, the hunk doesn't just contain the differing record pair. It may also
		// include context lines before and after the difference.

		// We get the before-the-difference record pairs from the tailbuf.
		// Conveniently, the tailbuf already contains the differing record pair.
		hunkPairs = tb.Slice(row-rd.cfg.Lines, row+1)

		// Now we need to get the after-the-difference record pairs. We look for a
		// sequence of non-differing (matching) record pairs, appending each
		// pair to hunkPairs. We stop when:
		//
		//  - we've found df.cfg.Lines of contiguous matching record pairs, or
		//  - we've reached the end of the record pairs, or
		//  - we've reached maxHunkRecords.
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
				//return errz.Err(ctx.Err())
			case rp, ok = <-rd.recPairs:
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

			if pairMatchSeq > rd.cfg.Lines || len(hunkPairs) >= rd.cfg.HunkMaxSize {
				break
			}
		}

		// OK, now we've got enough record pairs to generate the hunk.
		if err = rd.populateHunk(ctx, hnk, hunkPairs); err != nil {
			break
		}
	}

	if err == nil {
		err = errz.Err(ctx.Err())
	}

	rd.doc.Seal(err)
	return err
}

func (rd *recordDiffer) populateHunk(ctx context.Context, hnk *hunk, pairs []record.Pair) error {
	var (
		handleTbl1 = rd.td1.src.Handle + "." + rd.td1.tblName
		handleTbl2 = rd.td2.src.Handle + "." + rd.td2.tblName

		body1, body2 string
		err          error
	)

	recs1 := make([]record.Record, len(pairs))
	recs2 := make([]record.Record, len(pairs))
	for i := range pairs {
		recs1[i] = pairs[i].Rec1()
		recs2[i] = pairs[i].Rec2()
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var bodyErr error
		body1, bodyErr = renderRecords(gCtx, rd.cfg, rd.recw1.recMeta, recs1)
		return bodyErr
	})
	g.Go(func() error {
		var bodyErr error
		body2, bodyErr = renderRecords(gCtx, rd.cfg, rd.recw2.recMeta, recs2)
		return bodyErr
	})
	if err = g.Wait(); err != nil {
		return err
	}

	msg := fmt.Sprintf("table {%s}", rd.td1.tblName)
	var unified string
	unified, err = computeUnified(ctx, msg, handleTbl1, handleTbl2, rd.cfg.Lines, body1, body2)
	if err != nil {
		return err
	}

	// Trim the diff "file header"... ultimately, we should change computeUnified
	// to not return this (e.g. add an arg "noHeader=true")
	unified = stringz.TrimHead(unified, 2)

	hunkHeader, hunkBody, found := strings.Cut(unified, "\n")
	if !found {
		return errz.New("hunk header not found")
	}

	hunkHeader, err = adjustHunkOffset(hunkHeader, hnk.row)
	if err != nil {
		return err
	}

	hnk.header = hunkHeader
	hnk.body = hunkBody
	return nil
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
// The short form used when there's no surrounding lines (-U=0).
//
// Note that "-44,7 +44,7" means that the change is at line 44 and the number of
// lines compared is 7 (although 8 lines will be rendered, because the changed
// line is shown twice: the before and after versions of the line).
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
		return "", errz.Errorf("failed to parse hunk: %s", hunk)
	}

	i1 += offset
	i3 += offset

	return fmt.Sprintf(formatShort, i1, i3), nil
}
