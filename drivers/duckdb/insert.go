package duckdb

import (
	"context"
	"database/sql"

	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// getTableRecordMeta returns the record.Meta for the given columns of a table.
func (d *driveri) getTableRecordMeta(ctx context.Context, db sqlz.DB, tblName string, colNames []string) (
	record.Meta, error,
) {
	colTypes, err := d.TableColumnTypes(ctx, db, tblName, colNames)
	if err != nil {
		return nil, err
	}

	destCols, _, err := d.RecordMeta(ctx, colTypes, nil)
	if err != nil {
		return nil, err
	}

	return destCols, nil
}

// newStmtExecFunc returns a driver.StmtExecFunc that executes stmt and returns
// the number of rows affected.
func newStmtExecFunc(stmt *sql.Stmt) driver.StmtExecFunc {
	return func(ctx context.Context, args ...any) (int64, error) {
		res, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return 0, errw(err)
		}
		affected, err := res.RowsAffected()
		return affected, errw(err)
	}
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	numRows int,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	stmt, err := driver.PrepareInsertStmt(ctx, d, db, destTbl, destColsMeta.Names(), numRows)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

// NewBatchInsert implements driver.SQLDriver. DuckDB supports the standard
// multi-row INSERT syntax, so we delegate to driver.DefaultNewBatchInsert.
func (d *driveri) NewBatchInsert(ctx context.Context, msg string, db sqlz.DB,
	_ *source.Source, destTbl string, destColNames []string,
) (*driver.BatchInsert, error) {
	return driver.DefaultNewBatchInsert(ctx, msg, d, db, destTbl, destColNames)
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string,
	destColNames []string, where string,
) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	query, err := buildUpdateStmt(destTbl, destColNames, where)
	if err != nil {
		return nil, err
	}

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}
