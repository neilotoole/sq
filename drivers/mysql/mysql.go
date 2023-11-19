package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/neilotoole/sq/libsq/ast"

	"github.com/neilotoole/sq/libsq/core/tablefq"

	"github.com/neilotoole/sq/libsq/core/loz"

	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/samber/lo"

	"github.com/go-sql-driver/mysql"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/retry"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/xo/dburl"
)

const (
	// Type is the MySQL source driver type.
	Type = source.DriverType("mysql")
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

// ConnParams implements driver.SQLDriver.
// See: https://github.com/go-sql-driver/mysql#dsn-data-source-name.
func (d *driveri) ConnParams() map[string][]string {
	return map[string][]string{
		"allowAllFiles":            {"false", "true"},
		"allowCleartextPasswords":  {"false", "true"},
		"allowFallbackToPlaintext": {"false", "true"},
		"allowNativePasswords":     {"false", "true"},
		"allowOldPasswords":        {"false", "true"},
		"charset":                  nil,
		"checkConnLiveness":        {"true", "false"},
		"clientFoundRows":          {"false", "true"},
		"collation":                collations,
		"columnsWithAlias":         {"false", "true"},
		"connectionAttributes":     nil,
		"interpolateParams":        {"false", "true"},
		"loc":                      {"UTC"},
		"maxAllowedPackage":        {"0", "67108864"},
		"multiStatements":          {"false", "true"},
		"parseTime":                {"false", "true"},
		"readTimeout":              {"0"},
		"rejectReadOnly":           {"false", "true"},
		"timeout":                  nil,
		"tls":                      {"false", "true", "skip-verify", "preferred"},
		"writeTimeout":             {"0"},
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
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
		DefaultPort: 3306,
	}
}

// Dialect implements driver.Driver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           Type,
		Placeholders:   placeholders,
		Enquote:        stringz.BacktickQuote,
		IntBool:        true,
		MaxBatchValues: 250,
		Ops:            dialect.DefaultOps(),
		Joins:          lo.Without(jointype.All(), jointype.FullOuter),
		Catalog:        false,
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
	r := render.NewDefaultRenderer()
	r.FunctionNames[ast.FuncNameSchema] = "DATABASE"
	r.FunctionOverrides[ast.FuncNameCatalog] = doRenderFuncCatalog
	return r
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(ctx context.Context, colTypes []*sql.ColumnType) (
	record.Meta, driver.NewRecordFunc, error,
) {
	recMeta, err := recordMetaFromColumnTypes(ctx, colTypes)
	if err != nil {
		return nil, nil, err
	}
	mungeFn := getNewRecordFunc(recMeta)
	return recMeta, mungeFn, nil
}

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	stmt := `CREATE SCHEMA ` + stringz.BacktickQuote(schemaName)
	_, err := db.ExecContext(ctx, stmt)
	return errz.Wrapf(err, "failed to create schema {%s}", schemaName)
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	stmt := `DROP SCHEMA ` + stringz.BacktickQuote(schemaName)
	_, err := db.ExecContext(ctx, stmt)
	return errz.Wrapf(err, "failed to drop schema {%s}", schemaName)
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *sqlmodel.TableDef) error {
	createStmt := buildCreateTableStmt(tblDef)

	_, err := db.ExecContext(ctx, createStmt)
	return errw(err)
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` ", tbl, col) + dbTypeNameFromKind(knd)

	_, err := db.ExecContext(ctx, q)
	if err != nil {
		return errz.Wrapf(errw(err), "alter table: failed to add column {%s} to table {%s}", col, tbl)
	}

	return nil
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&name); err != nil {
		return "", errw(err)
	}

	return name, nil
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)

	const q = `SHOW DATABASES`
	var schemas []string
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	for rows.Next() {
		var schema string
		if err = rows.Scan(&schema); err != nil {
			return nil, errz.Err(err)
		}
		schemas = append(schemas, schema)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	slices.Sort(schemas)

	return schemas, nil
}

// CurrentCatalog implements driver.SQLDriver. MySQL does not support catalogs,
// so this method returns an error.
func (d *driveri) CurrentCatalog(_ context.Context, _ sqlz.DB) (string, error) {
	return "", errz.New("mysql: catalog mechanism not supported")
}

// ListCatalogs implements driver.SQLDriver. MySQL does not support catalogs,
// so this method returns an error.
func (d *driveri) ListCatalogs(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New("mysql: catalog mechanism not supported")
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf("RENAME TABLE `%s` TO `%s`", tbl, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "alter table: failed to rename table {%s} to {%s}", tbl, newName)
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	q := fmt.Sprintf("ALTER TABLE `%s` RENAME COLUMN `%s` TO `%s`", tbl, col, newName)
	_, err := db.ExecContext(ctx, q)
	return errz.Wrapf(errw(err), "alter table: failed to rename column {%s.%s} to {%s}", tbl, col, newName)
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
			return 0, errw(err)
		}
		affected, err := res.RowsAffected()
		return affected, errw(err)
	}
}

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(ctx context.Context, db sqlz.DB,
	fromTable, toTable tablefq.T, copyData bool,
) (int64, error) {
	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s SELECT * FROM %s",
		tblfmt(toTable), tblfmt(fromTable))

	if !copyData {
		stmt += " WHERE 0"
	}

	affected, err := sqlz.ExecAffected(ctx, db, stmt)
	if err != nil {
		return 0, errw(err)
	}

	return affected, nil
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_NAME = ?`

	var count int64
	err := db.QueryRowContext(ctx, query, tbl).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count == 1, nil
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error {
	var stmt string

	if ifExists {
		stmt = fmt.Sprintf("DROP TABLE IF EXISTS %s RESTRICT", tblfmt(tbl))
	} else {
		stmt = fmt.Sprintf("DROP TABLE %s RESTRICT", tblfmt(tbl))
	}

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) ([]*sql.ColumnType, error) {
	const queryTpl = "SELECT %s FROM %s LIMIT 0"

	enquote := d.Dialect().Enquote
	tblNameQuoted := enquote(tblName)

	colsClause := "*"
	if len(colNames) > 0 {
		colNamesQuoted := loz.Apply(colNames, enquote)
		colsClause = strings.Join(colNamesQuoted, driver.Comma)
	}

	query := fmt.Sprintf(queryTpl, colsClause, tblNameQuoted)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
		return nil, errw(err)
	}

	err = rows.Err()
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)
		return nil, errw(err)
	}

	err = rows.Close()
	if err != nil {
		return nil, errw(err)
	}

	return colTypes, nil
}

func (d *driveri) getTableRecordMeta(ctx context.Context, db sqlz.DB, tblName string, colNames []string) (
	record.Meta, error,
) {
	colTypes, err := d.TableColumnTypes(ctx, db, tblName, colNames)
	if err != nil {
		return nil, err
	}

	destCols, _, err := d.RecordMeta(ctx, colTypes)
	if err != nil {
		return nil, err
	}

	return destCols, nil
}

// Open implements driver.PoolOpener.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Pool, error) {
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

	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, errw(err)
	}

	if src.Schema != "" {
		lg.FromContext(ctx).Debug("Setting default schema for MysQL connection",
			lga.Src, src,
			lga.Schema, src.Schema,
		)
		cfg.DBName = src.Schema
	}

	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, errw(err)
	}

	db := sql.OpenDB(connector)
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

	return errz.Wrapf(errw(db.PingContext(ctx)), "ping %s", src.Handle)
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
		return 0, errw(err)
	}
	defer lg.WarnIfFuncError(d.log, lgm.CloseDB, db.Close)

	// Not sure about the Tx requirements?
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, errw(err)
	}

	// For whatever reason, the "affected" count from TRUNCATE
	// always returns zero. So, we're going to synthesize it.
	var beforeCount int64
	err = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM `%s`", tbl)).Scan(&beforeCount)
	if err != nil {
		return 0, errz.Append(err, errw(tx.Rollback()))
	}

	affected, err = sqlz.ExecAffected(ctx, tx, fmt.Sprintf("TRUNCATE TABLE `%s`", tbl))
	if err != nil {
		return affected, errz.Append(err, errw(tx.Rollback()))
	}

	if affected != 0 {
		// Note: At the time of writing, this doesn't happen:
		// zero is always returned (which we don't like).
		// If this changes (driver changes?) then we'll revisit.
		d.log.Warn("Unexpectedly got non-zero rows affected from TRUNCATE", lga.Count, affected)
		return affected, errw(tx.Commit())
	}

	// TRUNCATE succeeded, therefore tbl is empty, therefore
	// the count of truncated rows must be beforeCount?
	return beforeCount, errw(tx.Commit())
}

// database implements driver.Pool.
type database struct {
	log  *slog.Logger
	db   *sql.DB
	src  *source.Source
	drvr *driveri
}

// DB implements driver.Pool.
func (d *database) DB(context.Context) (*sql.DB, error) {
	return d.db, nil
}

// SQLDriver implements driver.Pool.
func (d *database) SQLDriver() driver.SQLDriver {
	return d.drvr
}

// Source implements driver.Pool.
func (d *database) Source() *source.Source {
	return d.src
}

// TableMetadata implements driver.Pool.
func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	return getTableMetadata(ctx, d.db, tblName)
}

// SourceMetadata implements driver.Pool.
func (d *database) SourceMetadata(ctx context.Context, noSchema bool) (*source.Metadata, error) {
	return getSourceMetadata(ctx, d.src, d.db, noSchema)
}

// Close implements driver.Pool.
func (d *database) Close() error {
	d.log.Debug(lgm.CloseDB, lga.Handle, d.src.Handle)
	return errw(d.db.Close())
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
		return "", errz.Wrapf(errw(err), "invalid source location %s", src.RedactedLocation())
	}

	// Convert the location to the desired driver DSN.
	// Location: 	mysql://sakila:p_ssW0rd@localhost:3306/sqtest?allowOldPasswords=1
	// Driver DSN:	sakila:p_ssW0rd@tcp(localhost:3306)/sqtest?allowOldPasswords=1
	driverDSN := u.DSN

	myCfg, err := mysql.ParseDSN(driverDSN) // verify
	if err != nil {
		return "", errz.Wrapf(errw(err), "invalid source location: %s", driverDSN)
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

// tblfmt formats a table name for use in a query. The arg can be a string,
// or a tablefq.T.
func tblfmt[T string | tablefq.T](tbl T) string {
	tfq := tablefq.From(tbl)
	return tfq.Render(stringz.BacktickQuote)
}

func doRenderFuncCatalog(_ *render.Context, fn *ast.FuncNode) (string, error) {
	if fn.FuncName() != ast.FuncNameCatalog {
		// Shouldn't happen
		return "", errz.Errorf("expected %s function, got %q", ast.FuncNameCatalog, fn.FuncName())
	}

	const frag = `(SELECT CATALOG_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = DATABASE() LIMIT 1)`
	return frag, nil
}
