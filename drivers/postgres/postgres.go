// Package postgres implements the sq driver for postgres.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/dialect"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/jackc/pgx/v4"
	// Import jackc/pgx, which is our postgres driver.
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// Type is the postgres source driver type.
	Type = source.Type("postgres")

	// dbDrvr is the backing postgres SQL driver impl name.
	dbDrvr = "pgx"
)

// Provider is the postgres implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ source.Type) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

// driveri is the postgres implementation of driver.Driver.
type driveri struct {
	log *slog.Logger
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        Type,
		Description: "PostgreSQL",
		Doc:         "https://github.com/jackc/pgx",
		IsSQL:       true,
	}
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           Type,
		Placeholders:   placeholders,
		IdentQuote:     '"',
		Enquote:        stringz.DoubleQuote,
		MaxBatchValues: 1000,
		Ops:            dialect.DefaultOps(),
	}
}

func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)

	n := 1
	var sb strings.Builder
	for i := 0; i < numRows; i++ {
		sb.Reset()
		sb.WriteRune('(')
		for j := 1; j <= numCols; j++ {
			sb.WriteRune('$')
			sb.WriteString(strconv.Itoa(n))
			n++
			if j < numCols {
				sb.WriteString(driver.Comma)
			}
		}
		sb.WriteRune(')')
		rows[i] = sb.String()
	}

	return strings.Join(rows, driver.Comma)
}

// SQLBuilder implements driver.SQLDriver.
func (d *driveri) Renderer() *render.Renderer {
	r := render.NewDefaultRenderer()
	return r
}

// Open implements driver.Driver.
func (d *driveri) Open(_ context.Context, src *source.Source) (driver.Database, error) {
	db, err := sql.Open(dbDrvr, src.Location)
	if err != nil {
		return nil, errz.Err(err)
	}

	return &database{log: d.log, db: db, src: src, drvr: d}, nil
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != Type {
		return nil, errz.Errorf("expected source type {%s} but got {%s}", Type, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	dbase, err := d.Open(ctx, src)
	if err != nil {
		return err
	}

	defer lg.WarnIfCloseError(d.log, lgm.CloseDB, dbase.DB())

	return dbase.DB().Ping()
}

// Truncate implements driver.Driver.
// Note that Truncate makes a separate query to determine the
// row count of tbl before executing TRUNCATE. This row count
// query is not part of a transaction with TRUNCATE, although
// possibly it should be, as the number of rows may have changed.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64,
	err error,
) {
	// https://www.postgresql.org/docs/9.1/sql-truncate.html

	// RESTART IDENTITY and CASCADE/RESTRICT are from pg 8.2 onwards
	// FIXME: should first check the pg version for < pg8.2 support

	db, err := sql.Open(dbDrvr, src.Location)
	if err != nil {
		return affected, errz.Err(err)
	}

	affectedQuery := "SELECT COUNT(*) FROM " + idSanitize(tbl)
	err = db.QueryRowContext(ctx, affectedQuery).Scan(&affected)
	if err != nil {
		return 0, errz.Err(err)
	}

	truncateQuery := "TRUNCATE TABLE " + idSanitize(tbl)
	if reset {
		// if reset & src.DBVersion >= 8.2
		truncateQuery += " RESTART IDENTITY" // default is CONTINUE IDENTITY
	}
	// We could add RESTRICT here; alternative is CASCADE
	_, err = db.ExecContext(ctx, truncateQuery)
	if err != nil {
		return 0, errz.Err(err)
	}

	return affected, nil
}

// idSanitize sanitizes an identifier (such as table name). It will
// add surrounding quotes. For example:
//
//	table_name    -->    "table_name"
func idSanitize(s string) string {
	return pgx.Identifier([]string{s}).Sanitize()
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *sqlmodel.TableDef) error {
	stmt := buildCreateTableStmt(tblDef)

	_, err := db.ExecContext(ctx, stmt)
	return errz.Err(err)
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT CURRENT_SCHEMA()`).Scan(&name); err != nil {
		return "", errz.Err(err)
	}

	return name, nil
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf(`ALTER TABLE %q RENAME TO %q`, tbl, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(err, "alter table: failed to rename table {%s} to {%s}", tbl, newName)
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	q := fmt.Sprintf("ALTER TABLE %q RENAME COLUMN %q TO %q", tbl, col, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(err, "alter table: failed to rename column {%s.%s} to {%s}", tbl, col, newName)
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q ", tbl, col) + dbTypeNameFromKind(knd)

	_, err := db.ExecContext(ctx, q)
	if err != nil {
		return errz.Wrapf(err, "alter table: failed to add column {%s} to table {%s}", col, tbl)
	}

	return nil
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	numRows int,
) (*driver.StmtExecer, error) {
	// Note that the pgx driver doesn't support res.LastInsertId.
	// https://github.com/jackc/pgx/issues/411

	destColsMeta, err := d.getTableRecordMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	stmt, err := driver.PrepareInsertStmt(ctx, d, db, destTbl, destColsMeta.Names(), numRows)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt),
		destColsMeta)
	return execer, nil
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string,
	where string,
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

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt),
		destColsMeta)
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

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB, fromTable, toTable string, copyData bool) (int64, error) {
	stmt := fmt.Sprintf("CREATE TABLE %q AS TABLE %q", toTable, fromTable)

	if !copyData {
		stmt += " WITH NO DATA"
	}

	affected, err := sqlz.ExecAffected(ctx, db, stmt)
	if err != nil {
		return 0, errz.Err(err)
	}

	return affected, nil
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM information_schema.tables
WHERE table_name = $1`

	var count int64
	err := db.QueryRowContext(ctx, query, tbl).Scan(&count)
	if err != nil {
		return false, errz.Err(err)
	}

	return count == 1, nil
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl string, ifExists bool) error {
	var stmt string

	if ifExists {
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %q RESTRICT", tbl)
	} else {
		stmt = fmt.Sprintf("DROP TABLE %q RESTRICT", tbl)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errz.Err(err)
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	// We have to do some funky stuff to get the column types
	// from when the table has no rows.
	// https://stackoverflow.com/questions/8098795/return-a-value-if-no-record-is-found

	// If tblName is "person" and we want cols "username"
	// and "email", the query will look like:
	//
	// SELECT
	// 	(SELECT username FROM person LIMIT 1) AS username,
	// 	(SELECT email FROM person LIMIT 1) AS email
	// LIMIT 1;
	quote := string(d.Dialect().IdentQuote)
	tblNameQuoted := stringz.Surround(tblName, quote)

	var query string

	if len(colNames) == 0 {
		// When the table is empty, and colNames are not provided,
		// then we need to fetch the table col names independently.
		var err error
		colNames, err = getTableColumnNames(ctx, db, tblName)
		if err != nil {
			return nil, err
		}
	}

	var sb strings.Builder
	sb.WriteString("SELECT\n")
	for i, colName := range colNames {
		colNameQuoted := stringz.Surround(colName, quote)
		sb.WriteString(fmt.Sprintf("  (SELECT %s FROM %s LIMIT 1) AS %s", colNameQuoted, tblNameQuoted, colNameQuoted))
		if i < len(colNames)-1 {
			sb.WriteRune(',')
		}
		sb.WriteString("\n")
	}
	sb.WriteString("LIMIT 1")
	query = sb.String()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errz.Err(err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
		return nil, errz.Err(err)
	}

	err = rows.Err()
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
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

// getTableColumnNames consults postgres's information_schema.columns table,
// returning the names of the table's columns in ordinal order.
func getTableColumnNames(ctx context.Context, db sqlz.DB, tblName string) ([]string, error) {
	log := lg.FromContext(ctx)
	const query = `SELECT column_name FROM information_schema.columns
	WHERE table_schema = CURRENT_SCHEMA()
	AND table_name = $1
	ORDER BY ordinal_position`

	rows, err := db.QueryContext(ctx, query, tblName)
	if err != nil {
		return nil, errz.Err(err)
	}

	var colNames []string
	var colName string

	for rows.Next() {
		err = rows.Scan(&colName)
		if err != nil {
			lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)
			return nil, errz.Err(err)
		}

		colNames = append(colNames, colName)
	}

	if rows.Err() != nil {
		lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)
		return nil, errz.Err(err)
	}

	err = rows.Close()
	if err != nil {
		return nil, errz.Err(err)
	}

	return colNames, nil
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(colTypes []*sql.ColumnType) (sqlz.RecordMeta, driver.NewRecordFunc, error) {
	// The jackc/pgx driver doesn't report nullability (sql.ColumnType)
	// Apparently this is due to what postgres sends over the wire.
	// See https://github.com/jackc/pgx/issues/276#issuecomment-526831493
	// So, we'll set the scan type for each column to the nullable
	// version below.

	recMeta := make(sqlz.RecordMeta, len(colTypes))
	for i, colType := range colTypes {
		knd := kindFromDBTypeName(d.log, colType.Name(), colType.DatabaseTypeName())
		colTypeData := sqlz.NewColumnTypeData(colType, knd)
		setScanType(d.log, colTypeData, knd)
		recMeta[i] = sqlz.NewFieldMeta(colTypeData)
	}

	mungeFn := func(vals []any) (sqlz.Record, error) {
		// postgres doesn't need to do any special munging, so we
		// just use the default munging.
		rec, skipped := driver.NewRecordFromScanRow(recMeta, vals, nil)
		if len(skipped) > 0 {
			var skippedDetails []string

			for _, skip := range skipped {
				meta := recMeta[skip]
				skippedDetails = append(skippedDetails,
					fmt.Sprintf("[%d] %s: db(%s) --> kind(%s) --> scan(%s)",
						skip, meta.Name(), meta.DatabaseTypeName(), meta.Kind(), meta.ScanType()))
			}

			return nil, errz.Errorf("expected zero skipped cols but have %d:\n  %s",
				skipped, strings.Join(skippedDetails, "\n  "))
		}
		return rec, nil
	}

	return recMeta, mungeFn, nil
}

// database is the postgres implementation of driver.Database.
type database struct {
	log  *slog.Logger
	drvr *driveri
	db   *sql.DB
	src  *source.Source
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
	return getTableMetadata(ctx, d.DB(), tblName)
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	return getSourceMetadata(ctx, d.src, d.DB())
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debug(lgm.CloseDB, lga.Src, d.src)

	err := d.db.Close()
	if err != nil {
		return errz.Err(err)
	}
	return nil
}
