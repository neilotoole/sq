package libsq

import (
	"context"
	"database/sql"
	"sync"

	"github.com/neilotoole/lg"
	"go.uber.org/atomic"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/sqlz"
)

// DefaultRecordChSize is the default size of a record channel.
const DefaultRecordChSize = 100

// DBWriter implements RecordWriter, writing
// records to a database table.
type DBWriter struct {
	log      lg.Log
	wg       *sync.WaitGroup
	cancelFn context.CancelFunc
	destDB   driver.Database
	destTbl  string
	recordCh chan sqlz.Record
	written  *atomic.Int64
	errCh    chan error
	errs     []error

	// preWriteHook, when non-nil, is invoked by the Open method before any
	// records are written. This is useful when the recMeta or tx are
	// needed to perform actions before insertion, such as creating
	// the dest table on the fly.
	preWriteHook func(ctx context.Context, recMeta sqlz.RecordMeta, tx sqlz.DB) error
}

// NewDBWriter returns a new writer than implements RecordWriter.
// The writer writes records from recordCh to destTbl
// in destDB. The recChSize param controls the size of recordCh
// returned by the writer's Open method.
func NewDBWriter(log lg.Log, destDB driver.Database, destTbl string, recChSize int) *DBWriter {
	return &DBWriter{
		log:      log,
		destDB:   destDB,
		destTbl:  destTbl,
		recordCh: make(chan sqlz.Record, recChSize),
		errCh:    make(chan error, 3),
		written:  atomic.NewInt64(0),
		wg:       &sync.WaitGroup{},
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

	if w.preWriteHook != nil {
		err = w.preWriteHook(ctx, recMeta, tx)
		if err != nil {
			w.rollback(tx, err)
			return nil, nil, err
		}
	}

	inserter, err := w.destDB.SQLDriver().PrepareInsertStmt(ctx, tx, w.destTbl, recMeta.Names(), 1)
	if err != nil {
		w.rollback(tx, err)
		return nil, nil, err
	}

	w.wg.Add(1)
	go func() {
		defer func() {
			// When the inserter goroutine finishes:
			// - we close the errCh (and indicator that the writer is done)
			// - and mark the wg as done, which the Wait method depends upon.
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
					commitErr := errz.Err(tx.Commit())
					if commitErr != nil {
						w.log.Error(commitErr)
						w.addErrs(commitErr)
					} else {
						w.log.Debugf("Tx commit success for %s.%s", w.destDB.Source().Handle, w.destTbl)
					}
					return
				}

				// rec is not nil, therefore we write it out
				err = w.doInsert(ctx, inserter, rec)
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
	return w.written.Load(), errz.Combine(w.errs...)
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

func (w *DBWriter) doInsert(ctx context.Context, inserter *driver.StmtExecer, rec sqlz.Record) error {
	err := inserter.Munge(rec)
	if err != nil {
		return err
	}

	affected, err := inserter.Exec(ctx, rec...)
	if err != nil {
		// NOTE: in the scenario where we're inserting into
		//  a SQLite db, and there's multiple writers (inserters) to
		//  the same db, a "database is locked" error from SQLite is
		//  possible. See https://github.com/mattn/go-sqlite3/issues/274
		//  Perhaps there's a sensible way to handle such an error that
		//  could be tackled here.
		return errz.Err(err)
	}

	if affected != 1 {
		w.log.Warnf("expected 1 affected row for insert, but got %d", affected)
	}

	w.written.Add(affected)
	return nil
}
