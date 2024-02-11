package diff

import (
	"bytes"
	"context"
	"fmt"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/yamlw"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"io"
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

		if err = populateRecordDiff(ctx, ds.cfg, recDiff); err != nil {
			return err
		}

		if err = Print(ctx, ds.out, ds.cfg.pr, recDiff.header, recDiff.diff); err != nil {
			return err
		}
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
	go func() {
		err := libsq.ExecuteSLQ(ctx, qc, query1, recw1)
		if err != nil {
			cancelFn()
			select {
			case errCh <- err:
			default:
			}
		}
	}()
	go func() {
		err := libsq.ExecuteSLQ(ctx, qc, query2, recw2)
		if err != nil {
			cancelFn()
			select {
			case errCh <- err:
			default:
			}
		}
	}()
	go func() {
		err := handleDiffSink(ctx, ds)
		if err != nil {
			cancelFn()
			errCh <- err
		}
	}()

	var (
		rec1, rec2 record.Record
		i          = -1
		err        error
		//found      bool
	)

	for {
		i++
		rec1 = nil
		rec2 = nil
		err = nil

		select {
		case err = <-errCh:
		case <-ctx.Done():
			err = errz.Err(ctx.Err())
		case rec1 = <-recw1.recCh:
		}
		if err != nil {
			cancelFn()
			log.Error("Table diff", lga.Err, err)
			break
		}

		select {
		case err = <-errCh:
		case <-ctx.Done():
			err = errz.Err(ctx.Err())
		case rec2 = <-recw2.recCh:
		}
		if err != nil {
			cancelFn()
			log.Error("Table diff", lga.Err, err)
			break
		}

		if rec1 == nil && rec2 == nil {
			// End of data, no diff found
			break
		}

		rp := recPair{
			row:   i,
			rec1:  rec1,
			rec2:  rec2,
			equal: record.Equal(rec1, rec2),
		}

		ds.diffs <- rp
		//
		//if record.Equal(rec1, rec2) {
		//	continue
		//}
		//
		//// We've got a diff!
		//log.Debug("Found a table diff", "row", i)
		//found = true
		//break
	}

	if err != nil {
		return err
	}

	//if !found {
	//	return nil //nolint:nilnil
	//}
	//
	//recDiff := &recordDiff{
	//	td1:      td1,
	//	td2:      td2,
	//	recMeta1: recw1.recMeta,
	//	recMeta2: recw2.recMeta,
	//	rec1:     rec1,
	//	rec2:     rec2,
	//	row:      i,
	//}
	//
	//if err = populateRecordDiff(ctx, lines, ru.Writers.OutPrinting, recDiff); err != nil {
	//	return err
	//}

	return nil
}

//nolint:unused
func populateRecordDiff(ctx context.Context, cfg *Config, recDiff *recordDiff) error {

	var (
		handleTbl1 = recDiff.td1.src.Handle + "." + recDiff.td1.tblName
		handleTbl2 = recDiff.td2.src.Handle + "." + recDiff.td2.tblName

		body1, body2 string
		err          error
	)

	if body1, err = renderRecords(ctx, cfg, recDiff.recMeta1, []record.Record{recDiff.rec1}); err != nil {
		return err
	}
	if body2, err = renderRecords(ctx, cfg, recDiff.recMeta2, []record.Record{recDiff.rec2}); err != nil {
		return err
	}

	//if body1, err = renderRecord2YAML(ctx, pr, recDiff.recMeta1, recDiff.rec1); err != nil {
	//	return err
	//}
	//if body2, err = renderRecord2YAML(ctx, pr, recDiff.recMeta1, recDiff.rec2); err != nil {
	//	return err
	//}

	msg := fmt.Sprintf("table {%s}", recDiff.td1.tblName)
	recDiff.diff, err = computeUnified(ctx, msg, handleTbl1, handleTbl2, cfg.Lines, body1, body2)
	if err != nil {
		return err
	}

	//recDiff.header = fmt.Sprintf("sq diff %s %s | .[%d]", handleTbl1, handleTbl2, recDiff.row)

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

	//yw := yamlw.NewRecordWriter(buf, cfg.pr)
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
