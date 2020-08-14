// Package sqlserver implements the sq driver for SQL Server.
package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	mssql "github.com/denisenkom/go-mssqldb"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlbuilder"
	"github.com/neilotoole/sq/libsq/sqlmodel"
	"github.com/neilotoole/sq/libsq/sqlz"
	"github.com/neilotoole/sq/libsq/stringz"
)

const (
	// Type is the SQL Server source driver type.
	Type = source.Type("sqlserver")

	// dbDrvr is the backing SQL Server driver impl name.
	dbDrvr = "sqlserver"
)

// Provider is the SQL Server implementation of driver.Provider.
type Provider struct {
	Log lg.Log
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ source.Type) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type %q", typ)
	}

	return &driveri{log: p.Log}, nil
}

// driveri is the SQL Server implementation of driver.Driver.
type driveri struct {
	log lg.Log
}

// DriverMetadata implements driver.SQLDriver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        Type,
		Description: "Microsoft SQL Server",
		Doc:         "https://github.com/denisenkom/go-mssqldb",
		IsSQL:       true,
	}
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() driver.Dialect {
	return driver.Dialect{
		Type:           Type,
		Placeholders:   placeholders,
		Quote:          '"',
		MaxBatchValues: 1000,
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
			sb.WriteString("@p")
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
func (d *driveri) SQLBuilder() (sqlbuilder.FragmentBuilder, sqlbuilder.QueryBuilder) {
	return newFragmentBuilder(d.log), &queryBuilder{}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	db, err := sql.Open(dbDrvr, src.Location)
	if err != nil {
		return nil, errz.Err(err)
	}

	err = db.PingContext(ctx)
	if err != nil {
		d.log.WarnIfCloseError(db)
		return nil, errz.Err(err)
	}

	return &database{log: d.log, db: db, src: src, drvr: d}, nil
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != Type {
		return nil, errz.Errorf("expected source type %q but got %q", Type, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	db, err := sql.Open(dbDrvr, src.Location)
	if err != nil {
		return errz.Err(err)
	}

	defer d.log.WarnIfCloseError(db)

	err = db.PingContext(ctx)
	return errz.Err(err)
}

// Truncate implements driver.Driver. Due to a quirk of SQL Server, the
// operation is implemented in two statements. First "DELETE FROM tbl" to
// delete all rows. Then, if reset is true, the table sequence counter
// is reset via RESEED.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (affected int64, err error) {
	// https://docs.microsoft.com/en-us/sql/t-sql/statements/truncate-table-transact-sql?view=sql-server-ver15

	// When there are foreign key constraints on mssql tables,
	// it's not possible to TRUNCATE the table. An alternative is
	// to delete all rows and reseed the identity column.
	//
	//  DELETE FROM "table1"; DBCC CHECKIDENT ('table1', RESEED, 1);
	//
	// See: https://stackoverflow.com/questions/253849/cannot-truncate-table-because-it-is-being-referenced-by-a-foreign-key-constraint

	db, err := sql.Open(dbDrvr, src.Location)
	if err != nil {
		return 0, errz.Err(err)
	}
	defer d.log.WarnIfFuncError(db.Close)

	affected, err = sqlz.ExecResult(ctx, db, fmt.Sprintf("DELETE FROM %q", tbl))
	if err != nil {
		return affected, errz.Wrapf(err, "truncate: failed to delete from %q", tbl)
	}

	if reset {
		_, err = db.ExecContext(ctx, fmt.Sprintf("DBCC CHECKIDENT ('%s', RESEED, 1)", tbl))
		if err != nil {
			return affected, errz.Wrapf(err, "truncate: deleted %d rows from %q but RESEED failed", affected, tbl)
		}
	}

	return affected, nil
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string, colNames []string) ([]*sql.ColumnType, error) {
	// SQLServer has this unusual incantation for its LIMIT equivalent:
	//
	// SELECT username, email, address_id FROM person
	// ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY;
	const queryTpl = "SELECT %s FROM %s ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY"

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

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		d.log.WarnIfFuncError(rows.Close)
		return nil, errz.Err(err)
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

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(colTypes []*sql.ColumnType) (sqlz.RecordMeta, driver.NewRecordFunc, error) {
	recMeta := make([]*sqlz.FieldMeta, len(colTypes))
	for i, colType := range colTypes {
		kind := kindFromDBTypeName(d.log, colType.Name(), colType.DatabaseTypeName())
		colTypeData := sqlz.NewColumnTypeData(colType, kind)
		setScanType(colTypeData, kind)
		recMeta[i] = sqlz.NewFieldMeta(colTypeData)
	}

	mungeFn := func(vals []interface{}) (sqlz.Record, error) {
		// sqlserver doesn't need to do any special munging, so we
		// just use the default munging.
		rec, skipped := driver.NewRecordFromScanRow(recMeta, vals, nil)
		if len(skipped) > 0 {
			return nil, errz.Errorf("expected zero skipped cols but have %d", skipped)
		}
		return rec, nil
	}

	return recMeta, mungeFn, nil
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *sqlmodel.TableDef) error {
	stmt := buildCreateTableStmt(tblDef)

	_, err := db.ExecContext(ctx, stmt)
	return errz.Err(err)
}

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB, fromTable, toTable string, copyData bool) (int64, error) {
	var stmt string

	if copyData {
		stmt = fmt.Sprintf("SELECT * INTO %q FROM %q", toTable, fromTable)
	} else {
		stmt = fmt.Sprintf("SELECT TOP(0) * INTO %q FROM %q", toTable, fromTable)
	}

	affected, err := sqlz.ExecResult(ctx, db, stmt)
	if err != nil {
		return 0, errz.Err(err)
	}

	return affected, nil
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl string, ifExists bool) error {
	var stmt string

	if ifExists {
		stmt = fmt.Sprintf("IF OBJECT_ID('dbo.%s', 'U') IS NOT NULL DROP TABLE dbo.%q", tbl, tbl)
	} else {
		stmt = fmt.Sprintf("DROP TABLE dbo.%q", tbl)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errz.Err(err)
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string, numRows int) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableColsMeta(ctx, db, destTbl, destColNames)
	if err != nil {
		return nil, err
	}

	stmt, err := driver.PrepareInsertStmt(ctx, d, db, destTbl, destColsMeta.Names(), numRows)
	if err != nil {
		return nil, err
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt, db, destTbl), destColsMeta)
	return execer, nil
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(ctx context.Context, db sqlz.DB, destTbl string, destColNames []string, where string) (*driver.StmtExecer, error) {
	destColsMeta, err := d.getTableColsMeta(ctx, db, destTbl, destColNames)
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

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt, db, destTbl), destColsMeta)
	return execer, nil
}

func (d *driveri) getTableColsMeta(ctx context.Context, db sqlz.DB, tblName string, colNames []string) (sqlz.RecordMeta, error) {
	// SQLServer has this unusual incantation for its LIMIT equivalent:
	//
	// SELECT username, email, address_id FROM person
	// ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY;
	const queryTpl = "SELECT %s FROM %s ORDER BY (SELECT 0) OFFSET 0 ROWS FETCH NEXT 1 ROWS ONLY"

	dialect := d.Dialect()
	quote := string(dialect.Quote)
	tblNameQuoted := stringz.Surround(tblName, quote)
	colNamesQuoted := stringz.SurroundSlice(colNames, quote)
	colsJoined := strings.Join(colNamesQuoted, driver.Comma)

	query := fmt.Sprintf(queryTpl, colsJoined, tblNameQuoted)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errz.Err(err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		d.log.WarnIfFuncError(rows.Close)
		return nil, errz.Err(err)
	}

	if rows.Err() != nil {
		return nil, errz.Err(rows.Err())
	}

	destCols, _, err := d.RecordMeta(colTypes)
	if err != nil {
		d.log.WarnIfFuncError(rows.Close)
		return nil, errz.Err(err)
	}

	err = rows.Close()
	if err != nil {
		return nil, errz.Err(err)
	}

	return destCols, nil
}

// database implements driver.Database.
type database struct {
	log  lg.Log
	drvr *driveri
	db   *sql.DB
	src  *source.Source
}

var _ driver.Database = (*database)(nil)

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
	const query = `SELECT TABLE_CATALOG, TABLE_SCHEMA, TABLE_TYPE
FROM INFORMATION_SCHEMA.TABLES
WHERE TABLE_NAME = @p1`

	var catalog, schema, tblType string
	err := d.db.QueryRowContext(ctx, query, tblName).Scan(&catalog, &schema, &tblType)
	if err != nil {
		return nil, errz.Err(err)
	}

	return getTableMetadata(ctx, d.log, d.db, catalog, schema, tblName, tblType)
	//
	//srcMeta, err := d.SourceMetadata(ctx)
	//if err != nil {
	//	return nil, err
	//}
	//return source.TableFromSourceMetadata(srcMeta, tblName)
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	return getSourceMetadata(ctx, d.log, d.src, d.db)
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debugf("Close database: %s", d.src)

	return errz.Err(d.db.Close())
}

// newStmtExecFunc returns a StmtExecFunc that has logic to deal with
// the "identity insert" error. If the error is encountered, setIdentityInsert
// is called and stmt is executed again.
func newStmtExecFunc(stmt *sql.Stmt, db sqlz.DB, tbl string) driver.StmtExecFunc {
	return func(ctx context.Context, args ...interface{}) (int64, error) {
		res, err := stmt.ExecContext(ctx, args...)
		if err == nil {
			var affected int64
			affected, err = res.RowsAffected()
			return affected, errz.Err(err)
		}

		if !hasErrCode(err, errCodeIdentityInsert) {
			return 0, errz.Err(err)
		}

		idErr := setIdentityInsert(ctx, db, tbl, true)
		if idErr != nil {
			return 0, errz.Combine(err, idErr)
		}

		res, err = stmt.ExecContext(ctx, args...)
		if err != nil {
			return 0, errz.Err(err)
		}

		affected, err := res.RowsAffected()
		return affected, errz.Err(err)
	}
}

// setIdentityInsert enables (or disables) "identity insert" for tbl on db.
// SQLServer is fussy about inserting values to the identity col. This
// error can be returned from the driver:
//
//   mssql: Cannot insert explicit value for identity column in table 'payment' when IDENTITY_INSERT is set to OFF
//
// The solution is "SET IDENTITY_INSERT tbl ON".
//
// See: https://docs.microsoft.com/en-us/sql/t-sql/statements/set-identity-insert-transact-sql?view=sql-server-ver15
func setIdentityInsert(ctx context.Context, db sqlz.DB, tbl string, on bool) error {
	var mode = "ON"
	if !on {
		mode = "OFF"
	}

	query := fmt.Sprintf("SET IDENTITY_INSERT %q %s", tbl, mode)
	_, err := db.ExecContext(ctx, query)
	return errz.Wrapf(err, "failed to SET IDENTITY INSERT %s %s", tbl, mode)
}

// mssql error codes
// https://docs.microsoft.com/en-us/sql/relational-databases/errors-events/database-engine-events-and-errors?view=sql-server-ver15
const (
	errCodeIdentityInsert int32 = 544
	errCodeObjectNotExist int32 = 15009
)

// hasErrCode returns true if err (or its cause err) is
// of type mssql.Error and err.Number equals code.
func hasErrCode(err error, code int32) bool {
	if err, ok := errz.Cause(err).(mssql.Error); ok {
		return err.Number == code
	}
	return false
}
