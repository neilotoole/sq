package diff

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/tailbuf"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"io"
	"strings"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

type diffSink struct {
	td1, td2     *tableData
	recw1, recw2 *recWriter
	recPairs     chan record.Pair
	out          io.Writer
	cfg          *Config
}

type recordDiffer struct {
	cfg                *Config
	td1, td2           *tableData
	recMeta1, recMeta2 record.Meta
	tb                 *tailbuf.Buf[record.Pair]
	ha                 *hunkAssembler
}

// FIXME: maxHunkRecords should be configurable
const maxHunkRecords = 1000

func handleDiffSink(ctx context.Context, ds *diffSink) error {
	var err error
	tb := tailbuf.New[record.Pair](ds.cfg.Lines)
	rd := &recordDiffer{
		cfg: ds.cfg,
		tb:  tb,
		td1: ds.td1,
		td2: ds.td2,
		ha:  newHunkAssembler(),
	}

	rd.ha.header = fmt.Sprintf("sq diff %s %s", ds.td1, ds.td2)

	var pairs []record.Pair
	var differingRow int
	_ = pairs
	_ = differingRow

	var ok bool
	var rp record.Pair
	for row := 0; ctx.Err() == nil; row++ {

		select {
		case <-ctx.Done():
			return errz.Err(ctx.Err())
		case rp, ok = <-ds.recPairs:
		}

		if !ok {
			break
		}

		if row == 0 {
			rd.recMeta1 = ds.recw1.recMeta
			rd.recMeta2 = ds.recw2.recMeta
		}

		tb.Write(rp)

		if rp.Equal() {
			differingRow = -1
			continue
		}

		hnk := rd.ha.newHunk(row)
		// We've found a differing row.
		differingRow = row

		pairs = tb.Slice(row-ds.cfg.Lines, row+1) // tb.Count() +1

		var identicalCount int

		for {
			select {
			case <-ctx.Done():
				return errz.Err(ctx.Err())
			case rp, ok = <-ds.recPairs:
			}

			if !ok {
				break
			}
			row++
			tb.Write(rp)
			pairs = append(pairs, rp)
			if rp.Equal() {
				identicalCount++
			} else {
				identicalCount = 0
			}

			if identicalCount >= ds.cfg.Lines-1 || len(pairs) >= maxHunkRecords {
				break
			}
		}

		if err = rd.generateHunkDiff(ctx, hnk, pairs); err != nil {
			return err
		}
	}

	if err = ctx.Err(); err != nil {
		return errz.Err(err)
	}

	if err = Print(ctx, ds.out, ds.cfg.pr, rd.ha.header, rd.ha.Reader()); err != nil {
		return err
	}

	return nil
}

func (df *recordDiffer) generateHunkDiff(ctx context.Context, hnk *hunk, pairs []record.Pair) error {
	var (
		handleTbl1 = df.td1.src.Handle + "." + df.td1.tblName
		handleTbl2 = df.td2.src.Handle + "." + df.td2.tblName

		body1, body2 string
		err          error
	)

	recs1 := make([]record.Record, len(pairs))
	recs2 := make([]record.Record, len(pairs))
	for i := range pairs {
		recs1[i] = pairs[i].Rec1()
		recs2[i] = pairs[i].Rec2()
	}

	if body1, err = renderRecords(ctx, df.cfg, df.recMeta1, recs1); err != nil {
		return err
	}
	if body2, err = renderRecords(ctx, df.cfg, df.recMeta2, recs2); err != nil {
		return err
	}

	msg := fmt.Sprintf("table {%s}", df.td1.tblName)
	var unified string
	unified, err = computeUnified(ctx, msg, handleTbl1, handleTbl2, df.cfg.Lines, body1, body2)
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

// execTableDataDiff compares the row data in td1 and td2, writing
// the diff to ru.Out.
//
// See: https://github.com/neilotoole/sq/issues/353.
func execTableDataDiff(ctx context.Context, ru *run.Run, cfg *Config,
	td1, td2 *tableData,
) error {
	recBufSize := tuning.OptRecBufSize.Get(options.FromContext(ctx))

	qc := run.NewQueryContext(ru, nil)

	query1 := td1.src.Handle + "." + stringz.DoubleQuote(td1.tblName)
	query2 := td2.src.Handle + "." + stringz.DoubleQuote(td2.tblName)

	errCh := make(chan error, 5)
	recw1 := &recWriter{
		recCh: make(chan record.Record, recBufSize),
		errCh: errCh,
	}
	recw2 := &recWriter{
		recCh: make(chan record.Record, recBufSize),
		errCh: errCh,
	}

	ds := &diffSink{
		td1:      td1,
		td2:      td2,
		recw1:    recw1,
		recw2:    recw2,
		recPairs: make(chan record.Pair, recBufSize),
		out:      ru.Out,
		cfg:      cfg,
	}

	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)

	// Query DB, send records to recw1.
	go func() {
		if err := libsq.ExecuteSLQ(ctx, qc, query1, recw1); err != nil {
			errCh <- err
		}
	}()
	// Query DB, send records to recw2.
	go func() {
		if err := libsq.ExecuteSLQ(ctx, qc, query2, recw2); err != nil {
			errCh <- err
		}
	}()

	// Consume records from recw1 and recw2, and compare them.
	// Send record pairs to ds.diffs.
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
				close(ds.recPairs)
				return
			}

			rp := record.NewPair(i, rec1, rec2)
			ds.recPairs <- rp
		}
	}()

	diffDone := make(chan struct{})
	// Consume the record pairs from ds.diffs, and write the diffs to ds.out.
	go func() {
		defer close(diffDone)
		if err := handleDiffSink(ctx, ds); err != nil {
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
	case <-diffDone:
		// 3. The diff sink has finished, but it could have finished
		// because it's done, or because it errored. We need to check.
		select {
		case err = <-errCh:
			// ACHSCHUALLLY, the diff sink errored.
		default:
		}
	}

	// No matter what happened above, we cancel the context.
	cancelFn()
	return err
}

var _ libsq.RecordWriter = (*recWriter)(nil)

type recWriter struct {
	recCh   chan record.Record
	errCh   chan error
	recMeta record.Meta
}

// Open implements libsq.RecordWriter.
func (d *recWriter) Open(_ context.Context, _ context.CancelFunc, recMeta record.Meta,
) (recCh chan<- record.Record, errCh <-chan error, err error) {
	d.recMeta = recMeta
	return d.recCh, d.errCh, nil
}

// Wait implements libsq.RecordWriter.
func (d *recWriter) Wait() (written int64, err error) {
	// We don't actually use Stop(), so just return zero values.
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
