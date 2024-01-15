package libsq

import (
	"context"
	"database/sql"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// MsgIngestRecords is the typical message used with [libsq.NewDBWriter]
// to indicate that records are being ingested.
const MsgIngestRecords = "Ingesting records"

// DBWriter implements RecordWriter, writing
// records to a database table.
type DBWriter struct {
	msg      string
	wg       *sync.WaitGroup
	cancelFn context.CancelFunc
	destGrip driver.Grip
	destTbl  string
	recordCh chan record.Record
	bi       *driver.BatchInsert
	errCh    chan error
	errs     []error

	// preWriteHook, when non-nil, is invoked by the Open method before any
	// records are written. This is useful when the recMeta or tx are
	// needed to perform actions before insertion, such as creating
	// the dest table on the fly.
	preWriteHooks []DBWriterPreWriteHook
}

// DBWriterPreWriteHook is a function that is invoked before DBWriter
// begins writing.
type DBWriterPreWriteHook func(ctx context.Context, recMeta record.Meta, destGrip driver.Grip, tx sqlz.DB) error

// DBWriterCreateTableIfNotExistsHook returns a hook that
// creates destTblName if it does not exist.
func DBWriterCreateTableIfNotExistsHook(destTblName string) DBWriterPreWriteHook {
	return func(ctx context.Context, recMeta record.Meta, destGrip driver.Grip, tx sqlz.DB) error {
		db, err := destGrip.DB(ctx)
		if err != nil {
			return err
		}
		tblExists, err := destGrip.SQLDriver().TableExists(ctx, db, destTblName)
		if err != nil {
			return errz.Err(err)
		}

		if tblExists {
			return nil
		}

		destColNames := recMeta.Names()
		destColKinds := recMeta.Kinds()
		destTblDef := sqlmodel.NewTableDef(destTblName, destColNames, destColKinds)

		err = destGrip.SQLDriver().CreateTable(ctx, tx, destTblDef)
		if err != nil {
			return errz.Wrapf(err, "failed to create dest table %s.%s", destGrip.Source().Handle, destTblName)
		}

		return nil
	}
}

// NewDBWriter returns a new writer than implements RecordWriter.
// The writer writes records from recordCh to destTbl
// in destGrip. The recChSize param controls the size of recordCh
// returned by the writer's Open method.
func NewDBWriter(msg string, destGrip driver.Grip, destTbl string, recChSize int,
	preWriteHooks ...DBWriterPreWriteHook,
) *DBWriter {
	return &DBWriter{
		msg:           msg,
		destGrip:      destGrip,
		destTbl:       destTbl,
		recordCh:      make(chan record.Record, recChSize),
		errCh:         make(chan error, 3),
		wg:            &sync.WaitGroup{},
		preWriteHooks: preWriteHooks,
	}

	// Note: errCh has size 3 because that's the maximum number of
	// errs that could be sent. Frequently only one err is sent,
	// but sometimes there are additional errs, e.g. when
	// ctx is done, we send ctx.Err, followed by any rollback err.
}

// Open implements RecordWriter.
func (w *DBWriter) Open(ctx context.Context, cancelFn context.CancelFunc, recMeta record.Meta) (
	chan<- record.Record, <-chan error, error,
) {
	w.cancelFn = cancelFn

	db, err := w.destGrip.DB(ctx)
	if err != nil {
		return nil, nil, err
	}

	// REVISIT: tx could potentially be passed to NewDBWriter?
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, errz.Wrapf(err, "failed to open tx for %s.%s", w.destGrip.Source().Handle, w.destTbl)
	}

	for _, hook := range w.preWriteHooks {
		err = hook(ctx, recMeta, w.destGrip, tx)
		if err != nil {
			w.rollback(ctx, tx, err)
			return nil, nil, err
		}
	}

	batchSize := driver.MaxBatchRows(w.destGrip.SQLDriver(), len(recMeta.Names()))
	w.bi, err = driver.NewBatchInsert(
		ctx,
		w.msg,
		w.destGrip.SQLDriver(),
		tx,
		w.destTbl,
		recMeta.Names(),
		batchSize,
	)
	if err != nil {
		w.rollback(ctx, tx, err)
		return nil, nil, err
	}

	w.wg.Add(1)
	go func() {
		defer func() {
			// When the inserter goroutine finishes:
			// - we close errCh (indicates that the DBWriter is done)
			// - and mark wg as done, which the Wait method depends upon.
			close(w.errCh)
			w.wg.Done()
		}()

		for {
			select {
			case <-ctx.Done():
				// ctx is done (e.g. cancelled), so we're going to rollback.
				w.rollback(ctx, tx, ctx.Err())
				return

			case rec := <-w.recordCh:
				if rec == nil {
					// No more results on recordCh, it has been closed.
					// It's time to commit the tx.
					// Note that Commit automatically closes any stmts
					// that were prepared by tx.

					// Tell batch inserter that we're done sending records
					close(w.bi.RecordCh)

					err = <-w.bi.ErrCh // Wait for batch inserter to complete
					if err != nil {
						lg.FromContext(ctx).Error(err.Error())
						w.addErrs(err)
						w.rollback(ctx, tx, err)
						return
					}

					commitErr := errz.Err(tx.Commit())
					if commitErr != nil {
						lg.FromContext(ctx).Error(commitErr.Error())
						w.addErrs(commitErr)
					} else {
						lg.FromContext(ctx).Debug("Tx commit success",
							lga.Target, source.Target(w.destGrip.Source(), w.destTbl))
					}

					return
				}

				// rec is not nil, therefore we write it to the db
				err = w.doInsert(ctx, rec)
				if err != nil {
					w.rollback(ctx, tx, err)
					return
				}

				// Otherwise, we successfully wrote rec to tx.
				// Therefore continue to wait/select for the next
				// element on recordCh (or for recordCh to close)
				// or for ctx.Done indicating timeout or cancel etc.
			}
		}
	}()

	return w.recordCh, w.errCh, nil
}

// Wait implements RecordWriter.
func (w *DBWriter) Wait() (written int64, err error) {
	w.wg.Wait()
	if w.cancelFn != nil {
		w.cancelFn()
	}

	if w.bi != nil {
		written = w.bi.Written()
	}

	return written, errz.Combine(w.errs...)
}

// addErrs handles any non-nil err in errs by appending it to w.errs
// and sending it on w.errCh.
func (w *DBWriter) addErrs(errs ...error) {
	for _, err := range errs {
		if err != nil {
			w.errs = append(w.errs, err)
			w.errCh <- err
		}
	}
}

// rollback rolls back tx. Note that rollback or commit of the tx
// will close each of the tx's prepared statements, so we don't
// need to close those manually.
func (w *DBWriter) rollback(ctx context.Context, tx *sql.Tx, causeErrs ...error) {
	// Guaranteed to be at least one causeErr
	lg.FromContext(ctx).Error("failed to insert data: tx will rollback",
		lga.Target, w.destGrip.Source().Handle+"."+w.destTbl,
		lga.Err, causeErrs[0])

	rollbackErr := errz.Err(tx.Rollback())
	lg.WarnIfError(lg.FromContext(ctx), lgm.TxRollback, rollbackErr)

	w.addErrs(causeErrs...)
	w.addErrs(rollbackErr)
}

func (w *DBWriter) doInsert(ctx context.Context, rec record.Record) error {
	err := w.bi.Munge(rec)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err = <-w.bi.ErrCh:
		return err
	case w.bi.RecordCh <- rec:
		return nil
	}
}
