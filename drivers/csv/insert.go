package csv

import (
	"context"
	"encoding/csv"
	"errors"
	"io"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
)

// execInsert inserts the CSV records in readAheadRecs (followed by records
// from the csv.Reader) via recw. The caller should wait on recw to complete.
func execInsert(ctx context.Context, recw libsq.RecordWriter, recMeta sqlz.RecordMeta,
	readAheadRecs [][]string, r *csv.Reader,
) error {
	ctx, cancelFn := context.WithCancel(ctx)

	recordCh, errCh, err := recw.Open(ctx, cancelFn, recMeta)
	if err != nil {
		return err
	}
	defer close(recordCh)

	// Before we continue reading from CSV, we first write out
	// any CSV records we read earlier.
	for i := range readAheadRecs {
		rec := mungeCSV2InsertRecord(readAheadRecs[i])

		select {
		case err = <-errCh:
			cancelFn()
			return err
		case <-ctx.Done():
			cancelFn()
			return ctx.Err()
		case recordCh <- rec:
		}
	}

	var csvRecord []string
	for {
		csvRecord, err = r.Read()
		if errors.Is(err, io.EOF) {
			// We're done reading
			return nil
		}
		if err != nil {
			cancelFn()
			return errz.Wrap(err, "read from CSV data source")
		}

		rec := mungeCSV2InsertRecord(csvRecord)

		select {
		case err = <-errCh:
			cancelFn()
			return err
		case <-ctx.Done():
			cancelFn()
			return ctx.Err()
		case recordCh <- rec:
		}
	}
}

// mungeCSV2InsertRecord returns a new []any containing
// the values of the csvRec []string.
func mungeCSV2InsertRecord(csvRec []string) []any {
	a := make([]any, len(csvRec))
	for i := range csvRec {
		a[i] = csvRec[i]
	}
	return a
}

func createTblDef(tblName string, colNames []string, kinds []kind.Kind) *sqlmodel.TableDef {
	tbl := &sqlmodel.TableDef{Name: tblName}

	cols := make([]*sqlmodel.ColDef, len(colNames))
	for i := range colNames {
		cols[i] = &sqlmodel.ColDef{Table: tbl, Name: colNames[i], Kind: kinds[i]}
	}

	tbl.Cols = cols
	return tbl
}

// getRecMeta returns RecordMeta to use with RecordWriter.Open.
func getRecMeta(ctx context.Context, scratchDB driver.Database, tblDef *sqlmodel.TableDef) (sqlz.RecordMeta, error) {
	colTypes, err := scratchDB.SQLDriver().TableColumnTypes(ctx, scratchDB.DB(), tblDef.Name, tblDef.ColNames())
	if err != nil {
		return nil, err
	}

	destMeta, _, err := scratchDB.SQLDriver().RecordMeta(colTypes)
	if err != nil {
		return nil, err
	}

	return destMeta, nil
}
