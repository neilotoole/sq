package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/neilotoole/sq/libsq/core/record"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/retry"

	"github.com/neilotoole/sq/libsq/driver/dialect"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/go-sql-driver/mysql"
	"github.com/xo/dburl"

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
	// Type is the MySQL source driver type.
	Type = source.DriverType("mysql")

	// dbDrvr is the backing MySQL SQL driver impl name.
	dbDrvr = "mysql"
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the MySQL implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ source.DriverType) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the MySQL implementation of driver.Driver.
type driveri struct {
	log *slog.Logger
}

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	return getDBProperties(ctx, db)
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        Type,
		Description: "MySQL",
		Doc:         "https://github.com/go-sql-driver/mysql",
		IsSQL:       true,
	}
}

// Dialect implements driver.Driver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           Type,
		Placeholders:   placeholders,
		IdentQuote:     '`',
		Enquote:        stringz.BacktickQuote,
		IntBool:        true,
		MaxBatchValues: 250,
		Ops:            dialect.DefaultOps(),
	}
}

func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)
	for i := 0; i < numRows; i++ {
		rows[i] = "(" + stringz.RepeatJoin("?", numCols, driver.Comma) + ")"
	}
	return strings.Join(rows, driver.Comma)
}

// Renderer implements driver.SQLDriver.
func (d *driveri) Renderer() *render.Renderer {
	return render.NewDefaultRenderer()
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(colTypes []*sql.ColumnType) (record.Meta, driver.NewRecordFunc, error) {
	recMeta := recordMetaFromColumnTypes(d.log, colTypes)
	mungeFn := getNewRecordFunc(recMeta)
	return recMeta, mungeFn, nil
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *sqlmodel.TableDef) error {
	createStmt := buildCreateTableStmt(tblDef)

	_, err := db.ExecContext(ctx, createStmt)
	return errz.Err(err)
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` ", tbl, col) + dbTypeNameFromKind(knd)

	_, err := db.ExecContext(ctx, q)
	if err != nil {
		return errz.Wrapf(err, "alter table: failed to add column {%s} to table {%s}", col, tbl)
	}

	return nil
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&name); err != nil {
		return "", errz.Err(err)
	}

	return name, nil
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf("RENAME TABLE `%s` TO `%s`", tbl, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(err, "alter table: failed to rename table {%s} to {%s}", tbl, newName)
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	q := fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", tbl, col, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(err, "alter table: failed to rename column {%s.%s} to {%s}", tbl, col, newName)
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

	execer := driver.NewStmtExecer(stmt, newInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt), destColsMeta)
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

	execer := driver.NewStmtExecer(stmt, newInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt), destColsMeta)
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
	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s` SELECT * FROM `%s`", toTable, fromTable)

	if !copyData {
		stmt += " WHERE 0"
	}

	affected, err := sqlz.ExecAffected(ctx, db, stmt)
	if err != nil {
		return 0, errz.Err(err)
	}

	return affected, nil
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ?`

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
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS `%s` RESTRICT", tbl)
	} else {
		stmt = fmt.Sprintf("DROP TABLE `%s` RESTRICT", tbl)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errz.Err(err)
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	const queryTpl = "SELECT %s FROM %s LIMIT 0"

	dialect := d.Dialect()
	quote := string(dialect.IdentQuote)
	tblNameQuoted := dialect.Enquote(tblName)

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
) (record.Meta, error) {
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

// Open implements driver.DatabaseOpener.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	db, err := d.doOpen(ctx, src)
	if err != nil {
		return nil, err
	}

	if err = driver.OpeningPing(ctx, src, db); err != nil {
		return nil, err
	}

	return &database{log: d.log, db: db, src: src, drvr: d}, nil
}

func (d *driveri) doOpen(ctx context.Context, src *source.Source) (*sql.DB, error) {
	dsn, err := dsnFromLocation(src, true)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(dbDrvr, dsn)
	if err != nil {
		return nil, errz.Err(err)
	}

	driver.ConfigureDB(ctx, db, src.Options)
	return db, nil
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != Type {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", Type, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseDB, db)

	return errz.Wrapf(db.PingContext(ctx), "ping %s", src.Handle)
}

// Truncate implements driver.SQLDriver. Arg reset is
// always ignored: the identity value is always reset by
// the TRUNCATE statement.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, _ bool) (affected int64,
	err error,
) {
	// https://dev.mysql.com/doc/refman/8.0/en/truncate-table.html
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return 0, errz.Err(err)
	}
	defer lg.WarnIfFuncError(d.log, lgm.CloseDB, db.Close)

	// Not sure about the Tx requirements?
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, errz.Err(err)
	}

	// For whatever reason, the "affected" count from TRUNCATE
	// always returns zero. So, we're going to synthesize it.
	var beforeCount int64
	err = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tbl)).Scan(&beforeCount)
	if err != nil {
		return 0, errz.Append(err, errz.Err(tx.Rollback()))
	}

	affected, err = sqlz.ExecAffected(ctx, tx, fmt.Sprintf("TRUNCATE TABLE `%s`", tbl))
	if err != nil {
		return affected, errz.Append(err, errz.Err(tx.Rollback()))
	}

	if affected != 0 {
		// Note: At the time of writing, this doesn't happen:
		// zero is always returned (which we don't like).
		// If this changes (driver changes?) then we'll revisit.
		d.log.Warn("Unexpectedly got non-zero rows affected from TRUNCATE", lga.Count, affected)
		return affected, errz.Err(tx.Commit())
	}

	// TRUNCATE succeeded, therefore tbl is empty, therefore
	// the count of truncated rows must be beforeCount?
	return beforeCount, errz.Err(tx.Commit())
}

// database implements driver.Database.
type database struct {
	log  *slog.Logger
	db   *sql.DB
	src  *source.Source
	drvr *driveri
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
	return getTableMetadata(ctx, d.db, tblName)
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	return getSourceMetadata(ctx, d.src, d.db)
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debug(lgm.CloseDB, lga.Handle, d.src.Handle)
	return errz.Err(d.db.Close())
}

// hasErrCode returns true if err (or its cause error)
// is of type *mysql.MySQLError and err.Number equals code.
func hasErrCode(err error, code uint16) bool {
	if err == nil {
		return false
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == code
	}

	return false
}

// https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html
const (
	errNumTableNotExist = uint16(1146)
	errNumConCount      = uint16(1040)
)

func isErrTooManyConnections(err error) bool {
	return hasErrCode(err, errNumConCount)
}

// dsnFromLocation builds the mysql driver DSN from src.Location.
// If parseTime is true, the param "parseTime=true" is added. This
// is because of: https://stackoverflow.com/questions/29341590/how-to-parse-time-from-database/29343013#29343013
func dsnFromLocation(src *source.Source, parseTime bool) (string, error) {
	if !strings.HasPrefix(src.Location, "mysql://") || len(src.Location) < 10 {
		return "", errz.Errorf("invalid source location %s", src.RedactedLocation())
	}

	u, err := dburl.Parse(src.Location)
	if err != nil {
		return "", errz.Wrapf(err, "invalid source location %s", src.RedactedLocation())
	}

	// Convert the location to the desired driver DSN.
	// Location: 	mysql://sakila:p_ssW0rd@localhost:3306/sqtest?allowOldPasswords=1
	// Driver DSN:	sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?allowOldPasswords=1
	driverDSN := u.DSN

	myCfg, err := mysql.ParseDSN(driverDSN) // verify
	if err != nil {
		return "", errz.Wrapf(err, "invalid source location: %s", driverDSN)
	}

	myCfg.ParseTime = parseTime
	driverDSN = myCfg.FormatDSN()

	return driverDSN, nil
}

// doRetry executes fn with retry on isErrTooManyConnections.
func doRetry(ctx context.Context, fn func() error) error {
	maxRetryInterval := driver.OptMaxRetryInterval.Get(options.FromContext(ctx))
	return retry.Do(ctx, maxRetryInterval, fn, isErrTooManyConnections)
}
