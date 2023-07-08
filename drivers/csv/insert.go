package csv

import (
	"context"
	"encoding/csv"
	"errors"
	"io"

	"github.com/neilotoole/sq/libsq/core/record"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
)

// execInsert inserts the CSV records in readAheadRecs (followed by records
// from the csv.Reader) via recw. The caller should wait on recw to complete.
func execInsert(ctx context.Context, recw libsq.RecordWriter, recMeta record.Meta,
	mungers []kind.MungeFunc, readAheadRecs [][]string, r *csv.Reader,
) error {
	ctx, cancelFn := context.WithCancel(ctx)
	// We don't do "defer cancelFn" here. The cancelFn is passed
	// to recw.

	recordCh, errCh, err := recw.Open(ctx, cancelFn, recMeta)
	if err != nil {
		return err
	}
	defer close(recordCh)

	// Before we continue reading from CSV, we first write out
	// any CSV records we read earlier.
	for i := range readAheadRecs {
		var rec []any
		if rec, err = mungeCSV2InsertRecord(ctx, mungers, readAheadRecs[i]); err != nil {
			return err
		}

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

		rec, err := mungeCSV2InsertRecord(ctx, mungers, csvRecord)
		if err != nil {
			return err
		}

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
func mungeCSV2InsertRecord(ctx context.Context, mungers []kind.MungeFunc, csvRec []string) ([]any, error) {
	var err error
	a := make([]any, len(csvRec))
	for i := range csvRec {
		if i >= len(mungers) {
			lg.FromContext(ctx).Error("no munger for field", lga.Index, i, lga.Val, csvRec[i])
			// Maybe should panic here, or return an error?
			// But, in future we may be able to handle ragged-edge records,
			// so maybe logging the error is best.
			continue
		}

		if mungers[i] != nil {
			a[i], err = mungers[i](csvRec[i])
			if err != nil {
				return nil, err
			}
		} else {
			a[i] = csvRec[i]
		}
	}
	return a, nil
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

// getRecMeta returns record.Meta to use with RecordWriter.Open.
func getRecMeta(ctx context.Context, scratchDB driver.Database, tblDef *sqlmodel.TableDef) (record.Meta, error) {
	db, err := scratchDB.DB(ctx)
	if err != nil {
		return nil, err
	}

	colTypes, err := scratchDB.SQLDriver().TableColumnTypes(ctx, db, tblDef.Name, tblDef.ColNames())
	if err != nil {
		return nil, err
	}

	destMeta, _, err := scratchDB.SQLDriver().RecordMeta(ctx, colTypes)
	if err != nil {
		return nil, err
	}

	return destMeta, nil
}
