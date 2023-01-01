// Package sqlite3 implements the sq driver for SQLite.
// The backing SQL driver is mattn/sqlite3.
package sqlite3

import "C"

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // Import for side effect of loading the driver
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
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64,
	err error,
) {
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
	err := db.QueryRowContext(ctx, fmt.Sprintf("SELECT sql FROM sqlite_master WHERE type='table' AND name='%s'",
		fromTable)).Scan(&originTblCreateStmt)
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
		rec := newRecordFromScanRow(recMeta, vals)
		return rec, nil
	}

	return recMeta, mungeFn, nil
}

// newRecordFromScanRow iterates over the elements of the row slice
// from rows.Scan, and returns a new (record) slice, replacing any
// wrapper types such as sql.NullString with the unboxed value,
// and other similar sanitization. For example, it will
// make a copy of any sql.RawBytes. The row slice
// can be reused by rows.Scan after this function returns.
//
// Note that this function can modify the kind of the RecordMeta elements
// if the kind is currently unknown. That is, if meta[0].Kind() returns
// kind.Unknown, but this function detects that row[0] is an *int64, then
// the kind will be set to kind.Int.
func newRecordFromScanRow(meta sqlz.RecordMeta, row []any) (rec sqlz.Record) { //nolint:funlen,gocognit,gocyclo,cyclop
	rec = make([]any, len(row))

	for i := 0; i < len(row); i++ {
		if row[i] == nil {
			rec[i] = nil
			continue
		}

		// Dereference *any before the switch
		col := row[i]
		if ptr, ok := col.(*any); ok {
			col = *ptr
		}

		switch col := col.(type) {
		default:
			// Shouldn't happen
			// TODO: We really should log here
			rec[i] = col
			continue
		case nil:
			rec[i] = nil
		case *int64:
			sqlz.SetKindIfUnknown(meta, i, kind.Int)
			v := *col
			rec[i] = &v
		case int64:
			sqlz.SetKindIfUnknown(meta, i, kind.Int)
			rec[i] = &col
		case *float64:
			sqlz.SetKindIfUnknown(meta, i, kind.Float)
			v := *col
			rec[i] = &v
		case float64:
			sqlz.SetKindIfUnknown(meta, i, kind.Float)
			rec[i] = &col
		case *bool:
			sqlz.SetKindIfUnknown(meta, i, kind.Bool)
			v := *col
			rec[i] = &v
		case bool:
			sqlz.SetKindIfUnknown(meta, i, kind.Bool)
			rec[i] = &col
		case *string:
			sqlz.SetKindIfUnknown(meta, i, kind.Text)
			v := *col
			rec[i] = &v
		case string:
			sqlz.SetKindIfUnknown(meta, i, kind.Text)
			rec[i] = &col
		case *[]byte:
			if col == nil || *col == nil {
				rec[i] = nil
				continue
			}

			if meta[i].Kind() != kind.Bytes {
				// We only want to use []byte for kind.Bytes. Otherwise
				// switch to a string.
				s := string(*col)
				rec[i] = &s
				sqlz.SetKindIfUnknown(meta, i, kind.Text)
				continue
			}

			if len(*col) == 0 {
				v := []byte{}
				rec[i] = &v
			} else {
				dest := make([]byte, len(*col))
				copy(dest, *col)
				rec[i] = &dest
			}
			sqlz.SetKindIfUnknown(meta, i, kind.Bytes)
		case *sql.NullInt64:
			if col.Valid {
				v := col.Int64
				rec[i] = &v
			} else {
				rec[i] = nil
			}
			sqlz.SetKindIfUnknown(meta, i, kind.Int)
		case *sql.NullString:
			if col.Valid {
				v := col.String
				rec[i] = &v
			} else {
				rec[i] = nil
			}
			sqlz.SetKindIfUnknown(meta, i, kind.Text)
		case *sql.RawBytes:
			if col == nil || *col == nil {
				// Explicitly set rec[i] so that its type becomes nil
				rec[i] = nil
				continue
			}

			knd := meta[i].Kind()

			// If RawBytes is of length zero, there's no
			// need to copy.
			if len(*col) == 0 {
				if knd == kind.Bytes {
					v := []byte{}
					rec[i] = &v
				} else {
					// Else treat it as an empty string
					var s string
					rec[i] = &s
					sqlz.SetKindIfUnknown(meta, i, kind.Text)
				}

				continue
			}

			dest := make([]byte, len(*col))
			copy(dest, *col)

			if knd == kind.Bytes {
				rec[i] = &dest
			} else {
				str := string(dest)
				rec[i] = &str
				sqlz.SetKindIfUnknown(meta, i, kind.Text)
			}

		case *sql.NullFloat64:
			if col.Valid {
				v := col.Float64
				rec[i] = &v
			} else {
				rec[i] = nil
			}
			sqlz.SetKindIfUnknown(meta, i, kind.Float)
		case *sql.NullBool:
			if col.Valid {
				v := col.Bool
				rec[i] = &v
			} else {
				rec[i] = nil
			}
			sqlz.SetKindIfUnknown(meta, i, kind.Bool)
		case *sqlz.NullBool:
			// This custom NullBool type is only used by sqlserver at this time.
			// Possibly this code should skip this item, and allow
			// the sqlserver munge func handle the conversion?
			if col.Valid {
				v := col.Bool
				rec[i] = &v
			} else {
				rec[i] = nil
			}
			sqlz.SetKindIfUnknown(meta, i, kind.Bool)
		case *sql.NullTime:
			if col.Valid {
				v := col.Time
				rec[i] = &v
			} else {
				rec[i] = nil
			}
			sqlz.SetKindIfUnknown(meta, i, kind.Datetime)
		case *time.Time:
			v := *col
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Datetime)
		case time.Time:
			v := col
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Datetime)

		// REVISIT: We probably don't need any of the below cases
		// for sqlite?
		case *int:
			v := int64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case int:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case *int8:
			v := int64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case int8:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case *int16:
			v := int64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case int16:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case *int32:
			v := int64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case int32:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case *uint:
			v := int64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case uint:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case *uint8:
			v := int64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case uint8:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case *uint16:
			v := int64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case uint16:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case *uint32:
			v := int64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case uint32:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case *float32:
			v := float64(*col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)

		case float32:
			v := int64(col)
			rec[i] = &v
			sqlz.SetKindIfUnknown(meta, i, kind.Int)
		}

		if rec[i] != nil && meta[i].Kind() == kind.Decimal {
			// Drivers use varying types for numeric/money/decimal.
			// We want to standardize on string.
			switch col := rec[i].(type) {
			case *string:
				// Do nothing, it's already string

			case *[]byte:
				v := string(*col)
				rec[i] = &v

			case *float64:
				v := stringz.FormatFloat(*col)
				rec[i] = &v

			default:
				// Shouldn't happen
				v := fmt.Sprintf("%v", col)
				rec[i] = &v
			}
		}
	}

	return rec
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
	query := buildCreateTableStmt(tblDef)

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
func (d *driveri) AlterTableAddColumn(ctx context.Context, db *sql.DB, tbl, col string, kind kind.Kind) error {
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
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		newStmtExecFunc(stmt), destColsMeta)
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
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	// Given the dynamic behavior of sqlite's rows.ColumnTypes,
	// this query selects a single row, as that'll give us more
	// accurate column type info than no rows. For other db
	// impls, LIMIT can be 0.
	const queryTpl = "SELECT %s FROM %s LIMIT 1"

	dialect := d.Dialect()
	quote := string(dialect.Quote)
	tblNameQuoted := stringz.Surround(tblName, quote)

	colsClause := "*"
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

func (d *driveri) getTableRecordMeta(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) (sqlz.RecordMeta, error) {
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

	if d.closed {
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

// MungeLocation takes a location argument (as received from the user)
// and builds a sqlite3 location URL. Each of these forms are allowed:
//
//	sqlite3:///path/to/sakila.db	--> sqlite3:///path/to/sakila.db
//	sqlite3:sakila.db 				--> sqlite3:///current/working/dir/sakila.db
//	sqlite3:/sakila.db 				--> sqlite3:///sakila.db
//	sqlite3:./sakila.db 			--> sqlite3:///current/working/dir/sakila.db
//	sqlite3:sakila.db 				--> sqlite3:///current/working/dir/sakila.db
//	sakila.db						--> sqlite3:///current/working/dir/sakila.db
//	/path/to/sakila.db				--> sqlite3:///path/to/sakila.db
//
// The final form is particularly nice for shell completion etc.
func MungeLocation(loc string) (string, error) {
	loc2 := strings.TrimSpace(loc)
	if loc2 == "" {
		return "", errz.New("location must not be empty")
	}

	loc2 = strings.TrimPrefix(loc2, "sqlite3://")
	loc2 = strings.TrimPrefix(loc2, "sqlite3:")

	// Now we should be left with just a path, which could be
	// relative or absolute.
	u, err := url.Parse(loc2)
	if err != nil {
		return "", errz.Wrapf(err, "invalid location: %s", loc)
	}

	fp, err := filepath.Abs(u.Path)
	if err != nil {
		return "", errz.Wrapf(err, "invalid location: %s", loc)
	}

	u.Path = filepath.ToSlash(fp)

	u.Scheme = "sqlite3"
	return u.String(), nil
}
