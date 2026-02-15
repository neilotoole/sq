package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/atomic"

	chv2 "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// NewBatchInsert implements driver.SQLDriver. It uses clickhouse-go's native
// Batch API (PrepareBatch/Append/Send) instead of the standard multi-row
// INSERT approach (see driver.DefaultNewBatchInsert), which doesn't work with
// clickhouse-go due to its single-row parameter binding requirement.
//
// This method opens a separate native ClickHouse connection (via chv2.Open)
// rather than using the sql.DB connection from the db parameter. This is
// necessary because the native Batch API (PrepareBatch/Append/Send) is not
// exposed through the database/sql interface. The db parameter is still used
// to retrieve column metadata for value transformation (munging).
//
// ClickHouse does not support ACID transactions, so there is no transactional
// consistency to maintain between this connection and the caller's sql.DB.
// Each batch Send is atomic at the batch level—either all rows in a batch are
// inserted or none are.
func (d *driveri) NewBatchInsert(ctx context.Context, msg string, db sqlz.DB,
	src *source.Source, destTbl string, destColNames []string,
) (*driver.BatchInsert, error) {
	batchSize := d.Dialect().MaxBatchValues
	if err := sqlz.RequireSingleConn(db); err != nil {
		return nil, err
	}

	// Apply default port if not specified.
	loc, _, err := locationWithDefaultPort(src.Location)
	if err != nil {
		return nil, err
	}

	// Parse the DSN to get native connection options.
	opts, err := chv2.ParseDSN(loc)
	if err != nil {
		return nil, errz.Wrapf(err, "parse clickhouse DSN for batch insert")
	}

	// Open a native clickhouse connection for the batch API.
	// The standard database/sql connection doesn't expose PrepareBatch,
	// so a separate native connection is required.
	conn, err := chv2.Open(opts)
	if err != nil {
		return nil, errz.Wrapf(err, "open clickhouse native connection for batch insert")
	}

	// Get column metadata for the munge function.
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	mungeFn := driver.DefaultInsertMungeFunc(destTbl, destColsMeta)

	// Build the INSERT query for PrepareBatch.
	// Format: INSERT INTO `tbl` (`c1`, `c2`)
	quotedCols := make([]string, len(destColNames))
	for i, col := range destColNames {
		quotedCols[i] = stringz.BacktickQuote(col)
	}
	insertQuery := fmt.Sprintf("INSERT INTO %s (%s)",
		stringz.BacktickQuote(destTbl),
		strings.Join(quotedCols, ", "))

	pbar := progress.FromContext(ctx).NewUnitCounter(msg, "rec")
	recCh := make(chan []any, batchSize*8)
	errCh := make(chan error, 1)
	written := atomic.NewInt64(0)
	log := lg.FromContext(ctx)

	go func() {
		defer conn.Close()
		defer close(errCh)
		defer pbar.Stop()

		batch, batchErr := conn.PrepareBatch(ctx, insertQuery)
		if batchErr != nil {
			errCh <- errz.Wrapf(batchErr, "prepare clickhouse batch")
			return
		}

		rowCount := 0

		for {
			select {
			case <-ctx.Done():
				_ = batch.Abort()
				errCh <- ctx.Err()
				return

			case rec, ok := <-recCh:
				if !ok {
					// Channel closed: send any remaining rows.
					if rowCount > 0 {
						if sendErr := batch.Send(); sendErr != nil {
							errCh <- errz.Wrapf(sendErr, "clickhouse batch send (final)")
							return
						}
						written.Add(int64(rowCount))
						pbar.Incr(rowCount)
					}

					log.Debug("ClickHouse batch insert complete",
						lga.Target, src.Handle+"."+destTbl,
						lga.Count, written.Load())
					return
				}

				if appendErr := batch.Append(rec...); appendErr != nil {
					_ = batch.Abort()
					errCh <- errz.Wrapf(appendErr, "clickhouse batch append")
					return
				}
				rowCount++

				if rowCount == batchSize {
					if sendErr := batch.Send(); sendErr != nil {
						errCh <- errz.Wrapf(sendErr, "clickhouse batch send")
						return
					}
					written.Add(int64(rowCount))
					pbar.Incr(rowCount)
					rowCount = 0

					batch, batchErr = conn.PrepareBatch(ctx, insertQuery)
					if batchErr != nil {
						errCh <- errz.Wrapf(batchErr, "prepare clickhouse batch")
						return
					}
				}
			}
		}
	}()

	return driver.NewBatchInsert(recCh, errCh, written, mungeFn), nil
}
