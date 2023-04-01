// Package libsq implements the core sq functionality.
// The ExecuteSLQ function is the entrypoint for executing
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

	"github.com/neilotoole/lg"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// QueryContext encapsulates the context a SLQ query is executed within.
type QueryContext struct {
	// Sources is the set of sources.
	Sources *source.Set

	// DBOpener is used to open databases.
	DBOpener driver.DatabaseOpener

	// JoinDBOpener is used to open the joindb (if needed).
	JoinDBOpener driver.JoinDatabaseOpener

	// Args defines variables that are substituted into the query.
	// May be nil or empty.
	Args map[string]string
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
	Open(ctx context.Context, cancelFn context.CancelFunc,
		recMeta sqlz.RecordMeta) (recCh chan<- sqlz.Record, errCh <-chan error, err error)

	// Wait waits for the writer to complete and returns the number of
	// written rows and any error (which may be a multierr).
	// The written value may be non-zero even in the presence of an error.
	// If a cancelFn was passed to Open, it will be invoked before Wait returns.
	Wait() (written int64, err error)
}

// ExecuteSLQ executes the slq query, writing the results to recw.
// The caller is responsible for closing qc.
func ExecuteSLQ(ctx context.Context, log lg.Log, qc *QueryContext, query string, recw RecordWriter,
) error {
	ng, err := newEngine(ctx, log, qc, query)
	if err != nil {
		return err
	}

	return ng.execute(ctx, recw)
}

// SLQ2SQL simulates execution of a SLQ query, but instead of executing
// the resulting SQL query, that ultimate SQL is returned. Effectively it is
// equivalent to libsq.ExecuteSLQ, but without the execution.
func SLQ2SQL(ctx context.Context, log lg.Log, qc *QueryContext, query string,
) (targetSQL string, err error) {
	var ng *engine
	ng, err = newEngine(ctx, log, qc, query)
	if err != nil {
		return "", err
	}
	return ng.targetSQL, nil
}

// QuerySQL executes the SQL query against dbase, writing
// the results to recw. Note that QuerySQL may return
// before recw has finished writing, thus the caller may wish
// to wait for recw to complete.
// The caller is responsible for closing dbase.
func QuerySQL(ctx context.Context, log lg.Log, dbase driver.Database, recw RecordWriter, query string,
	args ...any,
) error {
	rows, err := dbase.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return errz.Wrapf(err, `SQL query against %s failed: %s`, dbase.Source().Handle, query)
	}
	defer log.WarnIfCloseError(rows)

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
	// to construct the RecordMeta which, amongst other things, is used to
	// show column header info to the user, which we still want to do even
	// for an empty table.
	//
	// The workaround is that we call rows.ColumnTypes before the call to
	// rows.Next, and if rows.Next returns true, we call rows.ColumnTypes
	// again to get the more-complete []ColumnType. If rows.Next returns
	// false, we still make use of the earlier partially-complete []ColumnType.
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return errz.Err(err)
	}

	hasNext := rows.Next()
	if rows.Err() != nil {
		return errz.Err(rows.Err())
	}

	if hasNext {
		colTypes, err = rows.ColumnTypes()
		if err != nil {
			return errz.Err(err)
		}
	}

	drvr := dbase.SQLDriver()
	recMeta, recFromScanRowFn, err := drvr.RecordMeta(colTypes)
	if err != nil {
		return err
	}

	// We create a new ctx to pass to recw.Open; we use
	// the new ctx/cancelFn to stop recw if a problem happens
	// in this function.
	ctx, cancelFn := context.WithCancel(ctx)
	recordCh, errCh, err := recw.Open(ctx, cancelFn, recMeta)
	if err != nil {
		cancelFn()
		return err
	}
	defer close(recordCh)

	// scanRow is used by rows.Scan to get the DB vals.
	// It's relatively expensive to invoke NewScanRow, as it uses
	// pkg reflect to build scanRow (and also some drivers munge
	// the scan types, e.g. switching to sql.NullString instead
	// of *string). Therefore we reuse scanRow for each call to rows.Scan.
	scanRow := recMeta.NewScanRow()

	for hasNext {
		var rec sqlz.Record

		err = rows.Scan(scanRow...)
		if err != nil {
			cancelFn()
			return errz.Wrapf(err, "query against %s", dbase.Source().Handle)
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
		i, err = sqlz.ValidRecord(recMeta, rec)
		if err != nil {
			cancelFn()
			return errz.Wrapf(err, "column [%d] (%s): unacceptable munged type %T", i, recMeta[i].Name(), rec[i])
		}

		// We've got our new Record, now we need to decide
		// what to do.
		select {
		// If ctx is done, then we just return, we're done.
		case <-ctx.Done():
			log.WarnIfError(ctx.Err())
			cancelFn()
			return ctx.Err()

		// If there's an err from the record writer, we
		// return that error and we're done. Note that the error
		// will be nil when the RecordWriter closes errCh on
		// successful completion.
		case err = <-errCh:
			log.WarnIfError(err)
			cancelFn()
			return err

		// Otherwise, we send the record to recordCh. When
		// that send completes, the loop begins again for the
		// next row.
		case recordCh <- rec:
		}

		hasNext = rows.Next()
	}

	// For extra safety, check rows.Err.
	if rows.Err() != nil {
		log.WarnIfError(err)
		cancelFn()
		return errz.Err(rows.Err())
	}

	return nil
}
