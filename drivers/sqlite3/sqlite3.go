// Package sqlite3 implements the sq driver for SQLite.
// The backing SQL driver is mattn/sqlite3.
package sqlite3

import "C"
import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/ast/sqlbuilder"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// Type is the sqlite3 source driver type.
	Type source.Type = "sqlite3"

	// dbDrvr is the backing sqlite3 SQL driver impl name.
	dbDrvr = "sqlite3"

	// Prefix is the scheme+separator value "sqlite3://".
	Prefix = "sqlite3://"
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the SQLite3 implementation of driver.Provider.
type Provider struct {
	Log lg.Log
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ source.Type) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type %q", typ)
	}

	return &driveri{log: d.Log}, nil
}

var _ driver.Driver = (*driveri)(nil)

// driveri is the SQLite3 implementation of driver.Driver.
type driveri struct {
	log lg.Log
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        Type,
		Description: "SQLite",
		Doc:         "https://github.com/mattn/go-sqlite3",
		IsSQL:       true,
	}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	d.log.Debug("Opening data source: ", src)

	dsn, err := PathFromLocation(src)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(dbDrvr, dsn)
	if err != nil {
		return nil, errz.Wrapf(err, "failed to open sqlite3 source with DSN %q", dsn)
	}

	return &database{log: d.log, db: db, src: src, drvr: d}, nil
}

// Truncate implements driver.Driver.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64, err error) {
	dsn, err := PathFromLocation(src)
	if err != nil {
		return 0, err
	}

	db, err := sql.Open(dbDrvr, dsn)
	if err != nil {
		return 0, errz.Err(err)
	}
	defer d.log.WarnIfFuncError(db.Close)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, errz.Err(err)
	}

	affected, err = sqlz.ExecAffected(ctx, tx, fmt.Sprintf("DELETE FROM %q", tbl))
	if err != nil {
		return affected, errz.Append(err, errz.Err(tx.Rollback()))
	}

	if reset {
		// First check that the sqlite_sequence table event exists. It
		// may not exist if there are no auto-increment columns?
		const q = `SELECT COUNT(name) FROM sqlite_master WHERE type='table' AND name='sqlite_sequence'`
		var count int64
		err = tx.QueryRowContext(ctx, q).Scan(&count)
		if err != nil {
			return 0, errz.Append(err, errz.Err(tx.Rollback()))
		}

		if count > 0 {
			_, err = tx.ExecContext(ctx, "UPDATE sqlite_sequence SET seq = 0 WHERE name = ?", tbl)
			if err != nil {
				return 0, errz.Append(err, errz.Err(tx.Rollback()))
			}
		}
	}

	return affected, errz.Err(tx.Commit())
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != Type {
		return nil, errz.Errorf("expected driver type %q but got %q", Type, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	dbase, err := d.Open(ctx, src)
	if err != nil {
		return err
	}
	defer d.log.WarnIfCloseError(dbase)

	return dbase.DB().Ping()
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() driver.Dialect {
	return driver.Dialect{
		Type:           Type,
		Placeholders:   placeholders,
		Quote:          '"',
		MaxBatchValues: 500,
	}
}

func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)
	for i := 0; i < numRows; i++ {
		rows[i] = "(" + stringz.RepeatJoin("?", numCols, driver.Comma) + ")"
	}
	return strings.Join(rows, driver.Comma)
}

// SQLBuilder implements driver.SQLDriver.
func (d *driveri) SQLBuilder() (sqlbuilder.FragmentBuilder, sqlbuilder.QueryBuilder) {
	return newFragmentBuilder(d.log), &sqlbuilder.BaseQueryBuilder{}
}

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB, fromTable, toTable string, copyData bool) (int64, error) {
	// Per https://stackoverflow.com/questions/12730390/copy-table-structure-to-new-table-in-sqlite3
	// It is possible to copy the table structure with a simple statement:
	//  CREATE TABLE copied AS SELECT * FROM mytable WHERE 0
	// However, this does not keep the type information as desired. Thus
	// we need to do something more complicated.

	var originTblCreateStmt string
	err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT sql FROM sqlite_master WHERE type='table' AND name='%s'", fromTable)).Scan(&originTblCreateStmt)
	if err != nil {
		return 0, errz.Err(err)
	}

	// A simple replace of the table name should work to mutate the
	// above CREATE stmt to use toTable instead of fromTable.
	destTblCreateStmt := strings.Replace(originTblCreateStmt, fromTable, toTable, 1)

	_, err = db.ExecContext(ctx, destTblCreateStmt)
	if err != nil {
		return 0, errz.Err(err)
	}

	if !copyData {
		return 0, nil
	}

	stmt := fmt.Sprintf("INSERT INTO %q SELECT * FROM %q", toTable, fromTable)
	affected, err := sqlz.ExecAffected(ctx, db, stmt)
	if err != nil {
		return 0, errz.Err(err)
	}

	return affected, nil
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(colTypes []*sql.ColumnType) (sqlz.RecordMeta, driver.NewRecordFunc, error) {
	recMeta, err := recordMetaFromColumnTypes(d.log, colTypes)
	if err != nil {
		return nil, nil, errz.Err(err)
	}

	mungeFn := func(vals []any) (sqlz.Record, error) {
		// sqlite3 doesn't need to do any special munging, so we
		// just use the default munging.
		rec, skipped := driver.NewRecordFromScanRow(recMeta, vals, nil)
		if len(skipped) > 0 {
			return nil, errz.Errorf("expected zero skipped cols but have %v", skipped)
		}
		return rec, nil
	}

	return recMeta, mungeFn, nil
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl string, ifExists bool) error {
	var stmt string

	if ifExists {
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %q", tbl)
	} else {
		stmt = fmt.Sprintf("DROP TABLE %q", tbl)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errz.Err(err)
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *sqlmodel.TableDef) error {
	query, err := buildCreateTableStmt(tblDef)
	if err != nil {
		return err
	}

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return errz.Err(err)
	}

	_, err = stmt.ExecContext(ctx)
	if err != nil {
		d.log.WarnIfCloseError(stmt)
		return errz.Err(err)
	}

	return errz.Err(stmt.Close())
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db *sql.DB, tbl string, col string, kind kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q ", tbl, col) + DBTypeForKind(kind)

	_, err := db.ExecContext(ctx, q)
	if err != nil {
		return errz.Wrapf(err, "alter table: failed to add column %q to table %q", col, tbl)
	}

	return nil
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM sqlite_master WHERE name = ? and type='table'`

	var count int64
	err := db.QueryRowContext(ctx, query, tbl).Scan(&count)
	if err != nil {
		return false, errz.Err(err)
	}

	return count == 1, nil
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string, numRows int) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	stmt, err := driver.PrepareInsertStmt(ctx, d, db, destTbl, destColsMeta.Names(), numRows)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string, where string) (*driver.StmtExecer, error) {
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
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt), destColsMeta)
	return execer, nil
}

func newStmtExecFunc(stmt *sql.Stmt) driver.StmtExecFunc {
	return func(ctx context.Context, args ...any) (int64, error) {
		res, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return 0, errz.Err(err)
		}
		affected, err := res.RowsAffected()
		return affected, errz.Err(err)
	}
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string, colNames []string) ([]*sql.ColumnType, error) {
	// Given the dynamic behavior of sqlite's rows.ColumnTypes,
	// this query selects a single row, as that'll give us more
	// accurate column type info than no rows. For other db
	// impls, LIMIT can be 0.
	const queryTpl = "SELECT %s FROM %s LIMIT 1"

	dialect := d.Dialect()
	quote := string(dialect.Quote)
	tblNameQuoted := stringz.Surround(tblName, quote)

	var colsClause = "*"
	if len(colNames) > 0 {
		colNamesQuoted := stringz.SurroundSlice(colNames, quote)
		colsClause = strings.Join(colNamesQuoted, driver.Comma)
	}

	query := fmt.Sprintf(queryTpl, colsClause, tblNameQuoted)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errz.Err(err)
	}

	// We invoke rows.ColumnTypes twice.
	// The first time is to cover the scenario where the table
	// is empty (no rows), so that we at least get some
	// column type info.
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		d.log.WarnIfFuncError(rows.Close)
		return nil, errz.Err(err)
	}

	// If the table does have rows, we invoke rows.ColumnTypes again,
	// as on this invocation the column type info will be more
	// accurate (col nullability will be reported etc).
	if rows.Next() {
		colTypes, err = rows.ColumnTypes()
		if err != nil {
			d.log.WarnIfFuncError(rows.Close)
			return nil, errz.Err(err)
		}
	}

	err = rows.Err()
	if err != nil {
		d.log.WarnIfFuncError(rows.Close)
		return nil, errz.Err(err)
	}

	err = rows.Close()
	if err != nil {
		return nil, errz.Err(err)
	}

	return colTypes, nil
}

func (d *driveri) getTableRecordMeta(ctx context.Context, db sqlz.DB, tblName string, colNames []string) (sqlz.RecordMeta, error) {
	colTypes, err := d.TableColumnTypes(ctx, db, tblName, colNames)
	if err != nil {
		return nil, err
	}

	destCols, _, err := d.RecordMeta(colTypes)
	if err != nil {
		return nil, err
	}

	return destCols, nil
}

// database implements driver.Database.
type database struct {
	log  lg.Log
	db   *sql.DB
	src  *source.Source
	drvr *driveri

	// DEBUG: closeMu and closed exist while debugging close behavior
	closeMu sync.Mutex
	closed  bool
}

// DB implements driver.Database.
func (d *database) DB() *sql.DB {
	return d.db
}

// SQLDriver implements driver.Database.
func (d *database) SQLDriver() driver.SQLDriver {
	return d.drvr
}

// Source implements driver.Database.
func (d *database) Source() *source.Source {
	return d.src
}

// TableMetadata implements driver.Database.
func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	return getTableMetadata(ctx, d.log, d.DB(), tblName)
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	// https://stackoverflow.com/questions/9646353/how-to-find-sqlite-database-file-version

	meta := &source.Metadata{Handle: d.src.Handle, SourceType: Type, DBDriverType: dbDrvr}

	dsn, err := PathFromLocation(d.src)
	if err != nil {
		return nil, err
	}

	const q = "SELECT sqlite_version(), (SELECT name FROM pragma_database_list ORDER BY seq LIMIT 1);"

	var schemaName string // typically "main"
	err = d.DB().QueryRowContext(ctx, q).Scan(&meta.DBVersion, &schemaName)
	if err != nil {
		return nil, errz.Err(err)
	}

	meta.DBProduct = "SQLite3 v" + meta.DBVersion

	fi, err := os.Stat(dsn)
	if err != nil {
		return nil, errz.Err(err)
	}

	meta.Size = fi.Size()
	meta.Name = fi.Name()
	meta.FQName = fi.Name() + "/" + schemaName
	meta.Location = d.src.Location

	meta.Tables, err = getAllTblMeta(ctx, d.log, d.db)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()

	//if !d.closed {
	//	debug.PrintStack()
	//}

	if d.closed {
		//panic( "SQLITE DB already closed")
		d.log.Warnf("SQLite DB already closed: %v", d.src)
		return nil
	}

	d.log.Debugf("Closing database: %s", d.src)
	err := errz.Err(d.db.Close())
	d.closed = true
	return err
}

// NewScratchSource returns a new scratch src. Effectively this
// function creates a new sqlite db file in the temp dir, and
// src points at this file. The returned clnup func closes that
// db file and deletes it.
func NewScratchSource(log lg.Log, name string) (src *source.Source, clnup func() error, err error) {
	name = stringz.SanitizeAlphaNumeric(name, '_')
	_, f, cleanFn, err := source.TempDirFile(name + ".sqlite")
	if err != nil {
		return nil, cleanFn, err
	}

	log.Debugf("created sqlite3 scratch data source file: %s", f.Name())

	src = &source.Source{
		Type:     Type,
		Handle:   source.ScratchHandle,
		Location: Prefix + f.Name(),
	}

	return src, cleanFn, nil
}

// PathFromLocation returns the absolute file path
// from the source location, which should have the "sqlite3://" prefix.
func PathFromLocation(src *source.Source) (string, error) {
	if src.Type != Type {
		return "", errz.Errorf("driver %q does not support %q", Type, src.Type)
	}

	if !strings.HasPrefix(src.Location, Prefix) {
		return "", errz.Errorf("sqlite3 source location must begin with %q but was: %s", Prefix, src.RedactedLocation())
	}

	loc := strings.TrimPrefix(src.Location, Prefix)
	if len(loc) < 2 {
		return "", errz.Errorf("sqlite3 source location is too short: %s", src.RedactedLocation())
	}

	loc = filepath.Clean(loc)
	return loc, nil
}
