package driver

import (
	"context"
	"math"

	"go.uber.org/atomic"

	"github.com/neilotoole/sq/libsq/core/debugz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// BatchInsert encapsulates inserting records to a db. The caller sends
// (munged) records on recCh; the record values should be munged via
// the Munge method prior to sending. Records are written to db in
// batches of batchSize as passed to NewBatchInsert (the final batch may
// be less than batchSize). The caller must close recCh to indicate that
// all records have been sent, or cancel the ctx passed to
// NewBatchInsert to stop the insertion goroutine. Any error is returned
// on errCh. Processing is complete when errCh is closed: the caller
// must select on errCh.
type BatchInsert struct {
	// RecordCh is the channel that the caller sends records on. The
	// caller must close RecordCh when done.
	RecordCh chan<- []any

	// ErrCh returns any errors that occur during insert. ErrCh is
	// closed by BatchInsert when processing is complete.
	ErrCh <-chan error

	written *atomic.Int64

	mungeFn InsertMungeFunc
}

// Written returns the number of records inserted (at the time of
// invocation). For the final value, Written should be invoked after
// ErrCh is closed.
func (bi *BatchInsert) Written() int64 {
	return bi.written.Load()
}

// Munge should be invoked on every record before sending
// on RecordCh.
func (bi *BatchInsert) Munge(rec []any) error {
	return bi.mungeFn(rec)
}

// NewBatchInsert is the low-level constructor for BatchInsert. It assembles
// a BatchInsert from pre-created channels, an atomic write counter, and a
// munge function. The caller is responsible for starting a goroutine that
// reads from recCh, writes errors to errCh, and closes errCh when done.
//
// This constructor exists because BatchInsert has unexported fields (written,
// mungeFn) that external packages cannot set directly. Drivers that implement
// custom insertion logic (e.g. ClickHouse's native Batch API) use this
// constructor to wrap their own channel-based goroutine in a BatchInsert.
//
// For the standard multi-row INSERT approach, most drivers should instead
// delegate to DefaultNewBatchInsert, which creates the channels, starts the
// goroutine, and returns a ready-to-use BatchInsert.
//
// See also: DefaultNewBatchInsert, SQLDriver.NewBatchInsert.
func NewBatchInsert(recCh chan<- []any, errCh <-chan error,
	written *atomic.Int64, mungeFn InsertMungeFunc,
) *BatchInsert {
	return &BatchInsert{
		RecordCh: recCh,
		ErrCh:    errCh,
		written:  written,
		mungeFn:  mungeFn,
	}
}

// DefaultNewBatchInsert returns a new BatchInsert instance using the standard
// multi-row INSERT approach. It creates channels, prepares the INSERT
// statement via drvr.PrepareInsertStmt, and starts an internal goroutine
// that accumulates records into batches and executes them. The returned
// BatchInsert's internal goroutine is already running.
//
// Most SQL drivers (Postgres, MySQL, SQLite, SQL Server) delegate their
// SQLDriver.NewBatchInsert method to this function. Drivers that require
// custom insertion logic (e.g. ClickHouse) implement their own method
// and use the lower-level NewBatchInsert constructor instead.
//
// Note that the db arg must guarantee a single connection: that is,
// it must be a sql.Conn or sql.Tx. Otherwise, an error is returned.
//
// See also: NewBatchInsert, SQLDriver.NewBatchInsert.
//
//nolint:gocognit
func DefaultNewBatchInsert(ctx context.Context, msg string, drvr SQLDriver, db sqlz.DB,
	destTbl string, destColNames []string, batchSize int,
) (*BatchInsert, error) {
	log := lg.FromContext(ctx)

	if err := sqlz.RequireSingleConn(db); err != nil {
		return nil, err
	}

	pbar := progress.FromContext(ctx).NewUnitCounter(msg, "rec")

	recCh := make(chan []any, batchSize*8)
	errCh := make(chan error, 1)
	rowLen := len(destColNames)

	inserter, err := drvr.PrepareInsertStmt(ctx, db, destTbl, destColNames, batchSize)
	if err != nil {
		return nil, err
	}

	bi := &BatchInsert{RecordCh: recCh, ErrCh: errCh, written: atomic.NewInt64(0), mungeFn: inserter.mungeFn}

	go func() {
		// vals holds rows of values as a single slice. That is, vals is
		// a bunch of record fields appended to one big slice to pass
		// as args to the INSERT statement
		vals := make([]any, 0, rowLen*batchSize)

		var rec []any
		var affected int64

		defer func() {
			pbar.Stop()

			if inserter != nil {
				if err == nil {
					// If no pre-existing error, any inserter.Close error
					// becomes the error.
					err = errz.Err(inserter.Close())
				} else {
					// If there's already an error, we just log any
					// error from inserter.Close: the pre-existing error
					// is the primary concern.
					lg.WarnIfError(log, lgm.CloseDBStmt, errz.Err(inserter.Close()))
				}
			}

			if err != nil {
				errCh <- err
			}

			close(errCh)
		}()

		for {
			rec = nil //nolint:wastedassign

			select {
			case <-ctx.Done():
				err = ctx.Err()
				return
			case rec = <-recCh:
			}

			if rec != nil {
				if len(rec) != rowLen {
					err = errz.Errorf("batch insert: record should have %d values but found %d", rowLen, len(rec))
					return
				}

				vals = append(vals, rec...)
			}

			if len(vals) == 0 {
				// Nothing to do here, we're done
				return
			}

			if len(vals)/rowLen == batchSize { // We've got a full batch to send
				affected, err = inserter.Exec(ctx, vals...)
				if err != nil {
					return
				}

				bi.written.Add(affected)
				pbar.Incr(int(affected))
				debugz.DebugSleep(ctx)

				if rec == nil {
					// recCh is closed (coincidentally exactly on the
					// batch size), so we're successfully done.
					return
				}

				// reset vals for the next batch
				vals = vals[0:0]
				continue
			}

			if rec != nil {
				// recCh is not closed, so we loop to accumulate more records
				continue
			}

			// If we get this far, it means that rec is nil (indicating
			// no more records), but the number of remaining records
			// to write is less than batchSize. So, we'll need a new
			// inserter to write the remaining records.

			// First, close the existing full-batch-size inserter
			if inserter != nil {
				err = errz.Err(inserter.Close())
				inserter = nil
				if err != nil {
					return
				}
			}

			inserter, err = drvr.PrepareInsertStmt(ctx, db, destTbl, destColNames, len(vals)/rowLen)
			if err != nil {
				return
			}

			affected, err = inserter.Exec(ctx, vals...)
			if err != nil {
				return
			}

			bi.written.Add(affected)
			pbar.Incr(int(affected))
			debugz.DebugSleep(ctx)

			// We're done
			return
		}
	}()

	return bi, nil
}

// MaxBatchRows returns the maximum number of rows allowed for a
// batch insert for drvr. Note that the returned value may differ
// for each database driver.
func MaxBatchRows(drvr SQLDriver, numCols int) int {
	return int(math.Ceil(float64(drvr.Dialect().MaxBatchValues) / float64(numCols)))
}
