package diff

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

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
func findRecordDiffOld(ctx context.Context, ru *run.Run, lines int,
	td1, td2 *tableData,
) (*recordDiff, error) {
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

	gCtx, cancelFn := context.WithCancel(ctx)
	go func() {
		err := libsq.ExecuteSLQ(gCtx, qc, query1, recw1)
		if err != nil {
			cancelFn()
			select {
			case errCh <- err:
			default:
			}
		}
	}()
	go func() {
		err := libsq.ExecuteSLQ(gCtx, qc, query2, recw2)
		if err != nil {
			cancelFn()
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	var (
		rec1, rec2 record.Record
		i          = -1
		err        error
		found      bool
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

		if record.Equal(rec1, rec2) {
			continue
		}

		// We've got a diff!
		log.Debug("Found a table diff", "row", i)
		found = true
		break
	}

	if err != nil {
		return nil, err
	}

	if !found {
		return nil, nil //nolint:nilnil
	}

	recDiff := &recordDiff{
		td1:      td1,
		td2:      td2,
		recMeta1: recw1.recMeta,
		recMeta2: recw2.recMeta,
		rec1:     rec1,
		rec2:     rec2,
		row:      i,
	}

	if err = populateRecordDiffOld(ctx, lines, ru.Writers.OutPrinting, recDiff); err != nil {
		return nil, err
	}

	return recDiff, nil
}

//nolint:unused
func populateRecordDiffOld(ctx context.Context, lines int, pr *output.Printing, recDiff *recordDiff) error {
	pr = pr.Clone()
	pr.EnableColor(false)

	var (
		handleTbl1 = recDiff.td1.src.Handle + "." + recDiff.td1.tblName
		handleTbl2 = recDiff.td2.src.Handle + "." + recDiff.td2.tblName

		body1, body2 string
		err          error
	)

	if body1, err = renderRecord2YAML(ctx, pr, recDiff.recMeta1, recDiff.rec1); err != nil {
		return err
	}
	if body2, err = renderRecord2YAML(ctx, pr, recDiff.recMeta1, recDiff.rec2); err != nil {
		return err
	}

	msg := fmt.Sprintf("table {%s}", recDiff.td1.tblName)
	recDiff.diff, err = computeUnified(ctx, msg, handleTbl1, handleTbl2, lines, body1, body2)
	if err != nil {
		return err
	}

	recDiff.header = fmt.Sprintf("sq diff %s %s | .[%d]", handleTbl1, handleTbl2, recDiff.row)

	return nil
}
