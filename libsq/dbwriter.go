package libsq

import (
	"context"
	"database/sql"
	"sync"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
)

// DBWriter implements RecordWriter, writing
// records to a database table.
type DBWriter struct {
	log      lg.Log
	wg       *sync.WaitGroup
	cancelFn context.CancelFunc
	destDB   driver.Database
	destTbl  string
	recordCh chan sqlz.Record
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
type DBWriterPreWriteHook func(ctx context.Context, recMeta sqlz.RecordMeta, destDB driver.Database, tx sqlz.DB) error

// DBWriterCreateTableIfNotExistsHook returns a hook that
// creates destTblName if it does not exist.
func DBWriterCreateTableIfNotExistsHook(destTblName string) DBWriterPreWriteHook {
	return func(ctx context.Context, recMeta sqlz.RecordMeta, destDB driver.Database, tx sqlz.DB) error {
		tblExists, err := destDB.SQLDriver().TableExists(ctx, destDB.DB(), destTblName)
		if err != nil {
			return errz.Err(err)
		}

		if tblExists {
			return nil
		}

		destColNames := recMeta.Names()
		destColKinds := recMeta.Kinds()
		destTblDef := sqlmodel.NewTableDef(destTblName, destColNames, destColKinds)

		err = destDB.SQLDriver().CreateTable(ctx, tx, destTblDef)
		if err != nil {
			return errz.Wrapf(err, "failed to create dest table %s.%s", destDB.Source().Handle, destTblName)
		}

		return nil
	}
}

// NewDBWriter returns a new writer than implements RecordWriter.
// The writer writes records from recordCh to destTbl
// in destDB. The recChSize param controls the size of recordCh
// returned by the writer's Open method.
func NewDBWriter(log lg.Log, destDB driver.Database, destTbl string, recChSize int, preWriteHooks ...DBWriterPreWriteHook) *DBWriter {
	return &DBWriter{
		log:           log,
		destDB:        destDB,
		destTbl:       destTbl,
		recordCh:      make(chan sqlz.Record, recChSize),
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
func (w *DBWriter) Open(ctx context.Context, cancelFn context.CancelFunc, recMeta sqlz.RecordMeta) (chan<- sqlz.Record, <-chan error, error) {
	w.cancelFn = cancelFn

	// REVISIT: tx could potentially be passed to NewDBWriter?
	tx, err := w.destDB.DB().BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, errz.Wrapf(err, "failed to open tx for %s.%s", w.destDB.Source().Handle, w.destTbl)
	}

	for _, hook := range w.preWriteHooks {
		err = hook(ctx, recMeta, w.destDB, tx)
		if err != nil {
			w.rollback(tx, err)
			return nil, nil, err
		}
	}

	batchSize := driver.MaxBatchRows(w.destDB.SQLDriver(), len(recMeta.Names()))
	w.bi, err = driver.NewBatchInsert(ctx, w.log, w.destDB.SQLDriver(), tx, w.destTbl, recMeta.Names(), batchSize)
	if err != nil {
		w.rollback(tx, err)
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
				w.rollback(tx, ctx.Err())
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
						w.log.Error(err)
						w.addErrs(err)
						w.rollback(tx, err)
						return
					}

					commitErr := errz.Err(tx.Commit())
					if commitErr != nil {
						w.log.Error(commitErr)
						w.addErrs(commitErr)
					} else {
						w.log.Debugf("Tx commit success for %s.%s", w.destDB.Source().Handle, w.destTbl)
					}

					return
				}

				// rec is not nil, therefore we write it to the db
				err = w.doInsert(ctx, rec)
				if err != nil {
					w.rollback(tx, err)
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
// will close all of the tx's prepared statements, so we don't
// need to close those manually.
func (w *DBWriter) rollback(tx *sql.Tx, causeErrs ...error) {
	// Guaranteed to be at least one causeErr
	w.log.Errorf("failed to insert to %s.%s: tx rollback due to: %s",
		w.destDB.Source().Handle, w.destTbl, causeErrs[0])

	rollbackErr := errz.Err(tx.Rollback())
	w.log.WarnIfError(rollbackErr)

	w.addErrs(causeErrs...)
	w.addErrs(rollbackErr)
}

func (w *DBWriter) doInsert(ctx context.Context, rec sqlz.Record) error {
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
