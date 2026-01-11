// Package libsq implements the core sq functionality.
// The ExecSLQ function is the entrypoint for executing
// a SLQ query, which may interact with several data sources.
// The QuerySQL function executes a SQL query against a single
// source. Both functions ultimately send their result records to
// a RecordWriter. Implementations of RecordWriter write records
// to a destination, such as a JSON or CSV file. The NewDBWriter
// function returns a RecordWriter that writes records to a
// database.
package libsq

import (
	"context"
	"errors"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// QueryContext encapsulates the context a SLQ query is executed within.
type QueryContext struct {
	// Collection is the set of sources.
	Collection *source.Collection

	// Grips mediates access to driver.Grip instances.
	Grips *driver.Grips

	// Args defines variables that are substituted into the query.
	// May be nil or empty.
	Args map[string]string

	// PreExecStmts are statements that are executed before the query.
	// These can be used for edge-case behavior, such as setting up
	// variables in the session. These stmts are typically loaded
	// from render.Fragments.PreExecStmts.
	//
	// See also: QueryContext.PostExecStmts.
	PreExecStmts []string

	// PostExecStmts are statements that are executed after the query.
	//
	// See also: QueryContext.PreExecStmts.
	PostExecStmts []string
}

// RecordWriter is the interface for writing records to a
// destination. The Open method returns a channel to
// which the records are sent. The Wait method allows
// the sender to wait for the writer to complete.
type RecordWriter interface {
	// Open opens the RecordWriter for writing records described
	// by recMeta, returning a non-nil err if the initial open
	// operation fails.
	//
	// Records received on recCh are written to the
	// destination, possibly buffered, until recCh is closed.
	// Therefore, the caller must close recCh to indicate that
	// all records have been sent, so that the writer can
	// perform any end-of-stream actions. The caller can use the Wait
	// method to wait for the writer to complete. The returned errCh
	// will also be closed when complete.
	//
	// Any underlying write error is sent on errCh,
	// at which point the writer is defunct. Thus it is the
	// responsibility of the sender to check errCh before
	// sending again on recordCh. Note that implementations
	// may send more than one error on errCh, and that errCh
	// will be closed when the writer completes. Note also that the
	// errors sent on errCh are accumulated internally by the writer and
	// returned from the Wait method (if more than one error, they
	// may be combined into a multierr).
	//
	// The caller can stop the RecordWriter by cancelling ctx.
	// When ctx is done, the writer shuts down processing
	// of recCh and returns ctx.Err on errCh  (possibly
	// with additional errors from the shutdown).
	//
	// If cancelFn is non-nil, it is invoked only by the writer's Wait method.
	// If the Open method itself returns an error, it is the caller's
	// responsibility to invoke cancelFn to prevent resource leakage.
	//
	// It is noted that the existence of the cancelFn param is an unusual
	// construction. This mechanism exists to enable a goroutine to wait
	// on the writer outside the function that invoked Open, without
	// having to pass cancelFn around.
	Open(ctx context.Context, cancelFn context.CancelFunc, recMeta record.Meta) (
		recCh chan<- record.Record, errCh <-chan error, err error)

	// Wait waits for the writer to complete and returns the number of
	// written rows and any error (which may be a multierr).
	// The written value may be non-zero even in the presence of an error.
	// If a cancelFn was passed to Open, it will be invoked before Wait returns.
	Wait() (written int64, err error)
}

// ExecSLQ executes the SLQ query, writing the results to recw.
// The caller is responsible for closing qc.
//
// Note differences between ExecSLQ and ExecSQL: ExecSLQ executes a SLQ
// statement (which is a sort of pipeline) which could result in multiple
// backend SQL commands being executed against several different sources. By
// contrast, ExecSQL executes SQL against a single source.
func ExecSLQ(ctx context.Context, qc *QueryContext, query string, recw RecordWriter) error {
	p, err := newPipeline(ctx, qc, query)
	if err != nil {
		return err
	}

	return p.execute(ctx, recw)
}

// SLQ2SQL simulates execution of a SLQ query, but instead of executing
// the resulting SQL query, that ultimate SQL is returned. Effectively it is
// equivalent to libsq.ExecSLQ, but without the execution.
func SLQ2SQL(ctx context.Context, qc *QueryContext, query string) (targetSQL string, err error) {
	p, err := newPipeline(ctx, qc, query)
	if err != nil {
		return "", err
	}
	return p.targetSQL, nil
}

// ExecSQL executes a SQL statement (DDL/DML) that doesn't return rows,
// such as CREATE TABLE, INSERT, UPDATE, DELETE, DROP TABLE, etc.
// It returns the number of rows affected. If db is non-nil, the statement
// is executed against it. Otherwise, the connection is obtained from grip.
// The caller is responsible for closing grip (and db, if non-nil).
//
// See also: QuerySQL.
func ExecSQL(ctx context.Context, grip driver.Grip, db sqlz.DB,
	stmt string, args ...any,
) (affected int64, err error) {
	log := lg.FromContext(ctx)
	errw := grip.SQLDriver().ErrWrapFunc()

	if db == nil {
		if db, err = grip.DB(ctx); err != nil {
			return 0, err
		}
	}

	bar := progress.FromContext(ctx).NewWaiter("Execute statement")
	result, err := db.ExecContext(ctx, stmt, args...)
	bar.Stop()
	if err != nil {
		err = errz.Wrapf(errw(err), `SQL stmt against %s failed: %s`, grip.Source().Handle, stmt)
		select {
		case <-ctx.Done():
			// If the context was canceled, it's probably more accurate
			// to just return the context error.
			log.Debug("Error received, but context was done", lga.Err, err)
			return 0, ctx.Err()
		default:
			return 0, err
		}
	}

	affected, err = result.RowsAffected()
	if err != nil {
		return 0, errw(err)
	}

	return affected, nil
}

// QuerySQL executes the SQL query, writing the results to recw. If db is
// non-nil, the query is executed against it. Otherwise, the connection is
// obtained from grip.
//
// Note that QuerySQL may return before recw has finished writing, thus the
// caller may wish to wait for recw to complete.
// The caller is responsible for closing grip (and db, if non-nil).
//
// See also: ExecSQL.
func QuerySQL(ctx context.Context, grip driver.Grip, db sqlz.DB,
	recw RecordWriter, query string, args ...any,
) error {
	log := lg.FromContext(ctx)
	errw := grip.SQLDriver().ErrWrapFunc()

	if db == nil {
		var err error
		if db, err = grip.DB(ctx); err != nil {
			return err
		}
	}

	bar := progress.FromContext(ctx).NewWaiter("Execute query")
	rows, err := db.QueryContext(ctx, query, args...)
	bar.Stop()
	if err != nil {
		err = errz.Wrapf(errw(err), `SQL query against %s failed: %s`, grip.Source().Handle, query)
		select {
		case <-ctx.Done():
			// If the context was canceled, it's probably more accurate
			// to just return the context error.
			log.Debug("Error received, but context was done", lga.Err, err)
			return ctx.Err()
		default:
			return err
		}
	}

	defer sqlz.CloseRows(log, rows)

	// This next part is a bit ugly.
	//
	// For some databases (specifically sqlite), a call to rows.ColumnTypes
	// before rows.Next is first invoked will always return nil for the
	// scan type of the columns. After rows.Next is first invoked, the
	// scan type will then be reported.
	//
	// UPDATE: As of mattn/go-sqlite3@v1.14.16 (and probably earlier)
	// it seems that this behavior may have changed. That is, it seems
	// that rows.ColumnTypes is now returning values even before the first
	// rows.Next call. It may now be possible to refactor the below code
	// to remove the double call to rows.ColumnTypes.
	//
	// However, there is a snag. Assume an empty table. A call to rows.Next
	// returns false, and a following call to rows.ColumnTypes will return
	// an error (because the rows.Next call closed rows). But we still need
	// the column type info even for an empty table, because it's needed
	// to construct the record.Meta which, amongst other things, is used to
	// show column header info to the user, which we still want to do even
	// for an empty table.
	//
	// The workaround is that we call rows.ColumnTypes before the call to
	// rows.Next, and if rows.Next returns true, we call rows.ColumnTypes
	// again to get the more-complete []ColumnType. If rows.Next returns
	// false, we still make use of the earlier partially-complete []ColumnType.
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return errw(err)
	}

	hasNext := rows.Next()
	if rows.Err() != nil {
		return errw(rows.Err())
	}

	if hasNext {
		colTypes, err = rows.ColumnTypes()
		if err != nil {
			return errw(err)
		}
	}

	drvr := grip.SQLDriver()
	recMeta, recFromScanRowFn, err := drvr.RecordMeta(ctx, colTypes)
	if err != nil {
		return errw(err)
	}

	// We create a new ctx to pass to recw.Open; we use
	// the new ctx/cancelFn to stop recw if a problem happens
	// in this function.
	ctx, cancelFn := context.WithCancel(ctx)
	recordCh, errCh, err := recw.Open(ctx, cancelFn, recMeta)
	if err != nil {
		cancelFn()
		return errw(err)
	}
	defer close(recordCh)

	// scanRow is used by rows.Scan to get the DB vals.
	// It's relatively expensive to invoke NewScanRow, as it uses
	// pkg reflect to build scanRow (and also some drivers munge
	// the scan types, e.g. switching to sql.NullString instead
	// of *string). Therefore we reuse scanRow for each call to rows.Scan.
	scanRow := recMeta.NewScanRow()

	for hasNext {
		var rec record.Record

		err = rows.Scan(scanRow...)
		if err != nil {
			cancelFn()
			return errz.Wrapf(errw(err), "query against %s", grip.Source().Handle)
		}

		// recFromScanRowFn returns a new Record with appropriate
		// copies of scanRow's data, thus freeing up scanRow
		// for reuse on the next call to rows.Scan.
		rec, err = recFromScanRowFn(scanRow)
		if err != nil {
			cancelFn()
			return err
		}

		// Note: ultimately we should be able to ditch this
		//  check when we have more confidence in the codebase.
		var i int
		i, err = record.Valid(rec)
		if err != nil {
			cancelFn()
			return errz.Wrapf(err, "column [%d] (%s): unacceptable munged type %T", i, recMeta[i].Name(), rec[i])
		}

		// We've got our new Record, now we need to decide
		// what to do.
		select {
		// If ctx is done, then we just return, we're done.
		case <-ctx.Done():
			if !errors.Is(context.Cause(ctx), errz.ErrStop) {
				// No need to log if it's errz.ErrStop, as it's a sentinel.
				lg.WarnIfError(log, lgm.CtxDone, ctx.Err())
			}
			cancelFn()
			return ctx.Err()

		// If there's an err from the record writer, we
		// return that error and we're done. Note that the error
		// will be nil when the RecordWriter closes errCh on
		// successful completion.
		case err = <-errCh:
			lg.WarnIfError(log, "write record", err)
			cancelFn()
			return errw(err)

		// Otherwise, we send the record to recordCh. When
		// that send completes, the loop begins again for the
		// next row.
		case recordCh <- rec:
		}

		hasNext = rows.Next()
	}

	// For extra safety, check rows.Err.
	if rows.Err() != nil {
		lg.WarnIfError(log, lgm.ReadDBRows, err)
		cancelFn()
		return errw(rows.Err())
	}

	return nil
}
