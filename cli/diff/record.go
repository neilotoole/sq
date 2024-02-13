package diff

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// recordDiff is a container for a single record diff.
//
//nolint:unused
type recordDiff struct {
	td1, td2           *tableData
	header             string
	diff               string
	recMeta1, recMeta2 record.Meta
	rec1, rec2         record.Record
	row                int
}

type diffSink struct {
	td1, td2     *tableData
	recw1, recw2 *recWriter
	diffs        chan recPair
	out          io.Writer
	cfg          *Config
}

type recPair struct {
	row        int
	rec1, rec2 record.Record
	equal      bool
}

func handleDiffSink(ctx context.Context, ds *diffSink) error {
	var err error

	rd := &recordDiffer{cfg: ds.cfg, buf: &bytes.Buffer{}}

	for rp := range ds.diffs {
		if rp.equal {
			continue
		}

		recDiff := &recordDiff{
			td1:      ds.td1,
			td2:      ds.td2,
			recMeta1: ds.recw1.recMeta,
			recMeta2: ds.recw2.recMeta,
			rec1:     rp.rec1,
			rec2:     rp.rec2,
			row:      rp.row,
		}

		if err = rd.populateRecordDiff(ctx, recDiff); err != nil {
			return err
		}
	}

	header := fmt.Sprintf("sq diff %s %s", ds.td1, ds.td2)
	if err = Print(ctx, ds.out, ds.cfg.pr, header, rd.buf.String()); err != nil {
		return err
	}

	return nil
}

// findRecordDiff compares the row data in td1 and td2, returning
// a recordDiff instance if there's a difference between the
// equivalent rows. The function stops when it finds the first difference.
//
// NOTE: findRecordDiff (and the functions it calls) are currently unused.
// Instead diff is using a naive implementation that renders all table
// data to text, and then diffs that text. That impl can be horribly
// inefficient for large result sets. findRecordDiff demonstrates one
// possibly better path. The code is left here as a guilty reminder
// to tackle this issue.
//
// See:https://github.com/neilotoole/sq/issues/353
//
//nolint:unused
func execTableDataDiff(ctx context.Context, ru *run.Run, cfg *Config,
	td1, td2 *tableData,
) error {
	const chSize = 100

	log := lg.FromContext(ctx).
		With("a", td1.src.Handle+"."+td1.tblName).
		With("b", td2.src.Handle+"."+td2.tblName)

	_ = log

	qc := run.NewQueryContext(ru, nil)

	query1 := td1.src.Handle + "." + stringz.DoubleQuote(td1.tblName)
	query2 := td2.src.Handle + "." + stringz.DoubleQuote(td2.tblName)

	errCh := make(chan error, 5)
	recw1 := &recWriter{
		recCh: make(chan record.Record, chSize),
		errCh: errCh,
	}
	recw2 := &recWriter{
		recCh: make(chan record.Record, chSize),
		errCh: errCh,
	}

	ds := &diffSink{
		td1:   td1,
		td2:   td2,
		recw1: recw1,
		recw2: recw2,
		diffs: make(chan recPair, 100),
		out:   ru.Out,
		cfg:   cfg,
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

		for i := 0; ; i++ {
			rec1 = nil
			rec2 = nil

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
				close(ds.diffs)
				return
			}

			rp := recPair{
				row:   i,
				rec1:  rec1,
				rec2:  rec2,
				equal: record.Equal(rec1, rec2),
			}

			ds.diffs <- rp
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

type recordDiffer struct {
	cfg *Config
	buf *bytes.Buffer
}

//nolint:unused
func (rd *recordDiffer) populateRecordDiff(ctx context.Context, recDiff *recordDiff) error {
	var (
		handleTbl1 = recDiff.td1.src.Handle + "." + recDiff.td1.tblName
		handleTbl2 = recDiff.td2.src.Handle + "." + recDiff.td2.tblName

		body1, body2 string
		err          error
	)

	if body1, err = renderRecords(ctx, rd.cfg, recDiff.recMeta1, []record.Record{recDiff.rec1}); err != nil {
		return err
	}
	if body2, err = renderRecords(ctx, rd.cfg, recDiff.recMeta2, []record.Record{recDiff.rec2}); err != nil {
		return err
	}

	//if body1, err = renderRecord2YAML(ctx, pr, recDiff.recMeta1, recDiff.rec1); err != nil {
	//	return err
	//}
	//if body2, err = renderRecord2YAML(ctx, pr, recDiff.recMeta1, recDiff.rec2); err != nil {
	//	return err
	//}

	msg := fmt.Sprintf("table {%s}", recDiff.td1.tblName)
	recDiff.diff, err = computeUnified(ctx, msg, handleTbl1, handleTbl2, rd.cfg.Lines, body1, body2)
	if err != nil {
		return err
	}

	recDiff.diff = stringz.TrimHead(recDiff.diff, 2)

	hunkHeader, hunkBody, found := strings.Cut(recDiff.diff, "\n")
	if !found {
		return errz.New("hunk header not found")
	}

	hunkHeader, err = adjustHunkOffset(hunkHeader, recDiff.row)
	if err != nil {
		return err
	}

	rd.buf.WriteString(hunkHeader)
	rd.buf.WriteRune('\n')
	rd.buf.WriteString(hunkBody)

	// recDiff.header = fmt.Sprintf("sq diff %s %s | .[%d]", handleTbl1, handleTbl2, recDiff.row)

	return nil
}

func renderRecords(ctx context.Context, cfg *Config, recMeta record.Meta, recs []record.Record) (string, error) {
	if len(recs) == 0 {
		return "", nil
	}

	pr := cfg.pr.Clone()
	pr.EnableColor(false)
	pr.ShowHeader = false
	buf := &bytes.Buffer{}
	recw := cfg.RecordWriterFn(buf, pr)

	// yw := yamlw.NewRecordWriter(buf, cfg.pr)
	if err := recw.Open(ctx, recMeta); err != nil {
		return "", err
	}
	if err := recw.WriteRecords(ctx, recs); err != nil {
		return "", err
	}
	if err := recw.Flush(ctx); err != nil {
		return "", err
	}
	if err := recw.Close(ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

//nolint:unused
func renderRecord2YAML(ctx context.Context, pr *output.Printing,
	recMeta record.Meta, rec record.Record,
) (string, error) {
	if rec == nil {
		return "", nil
	}

	buf := &bytes.Buffer{}
	yw := yamlw.NewRecordWriter(buf, pr)
	if err := yw.Open(ctx, recMeta); err != nil {
		return "", err
	}
	if err := yw.WriteRecords(ctx, []record.Record{rec}); err != nil {
		return "", err
	}
	if err := yw.Flush(ctx); err != nil {
		return "", err
	}
	if err := yw.Close(ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
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
