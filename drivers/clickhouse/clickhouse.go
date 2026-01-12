// Package clickhouse implements the sq driver for ClickHouse.
package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	_ "github.com/ClickHouse/clickhouse-go/v2" // ClickHouse driver

	"github.com/neilotoole/sq/libsq/ast"
	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (
	// Type is the ClickHouse driver type.
	Type = drivertype.ClickHouse

	// defaultPort is the default ClickHouse native protocol port.
	defaultPort = 9000
)

// Provider is the ClickHouse implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the ClickHouse implementation of driver.Driver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver.
func (d *driveri) ConnParams() map[string][]string {
	// ClickHouse connection parameters
	return map[string][]string{
		"dial_timeout":             {"10s"},
		"compress":                 {"true", "false", "lz4", "zstd", "gzip"},
		"secure":                   {"true", "false"},
		"skip_verify":              {"true", "false"},
		"connection_open_strategy": {"in_order", "round_robin"},
		"max_open_conns":           {"10"},
		"max_idle_conns":           {"5"},
		"conn_max_lifetime":        {"1h"},
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        Type,
		Description: "ClickHouse",
		Doc:         "https://github.com/ClickHouse/clickhouse-go",
		IsSQL:       true,
		DefaultPort: defaultPort,
	}
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           Type,
		Placeholders:   placeholders,
		Enquote:        stringz.BacktickQuote,
		ExecModeFor:    dialect.DefaultExecModeFor,
		MaxBatchValues: 10000, // ClickHouse handles large batches well
		Ops:            dialect.DefaultOps(),
		Joins:          jointype.All(),
	}
}

func placeholders(numCols, numRows int) string {
	// ClickHouse uses ? for placeholders
	rows := make([]string, numRows)

	var sb strings.Builder
	for i := 0; i < numRows; i++ {
		sb.Reset()
		sb.WriteRune('(')
		for j := 0; j < numCols; j++ {
			sb.WriteRune('?')
			if j < numCols-1 {
				sb.WriteString(driver.Comma)
			}
		}
		sb.WriteRune(')')
		rows[i] = sb.String()
	}

	return strings.Join(rows, driver.Comma)
}

// Renderer implements driver.SQLDriver.
func (d *driveri) Renderer() *render.Renderer {
	r := render.NewDefaultRenderer()
	r.FunctionNames[ast.FuncNameSchema] = "currentDatabase"
	r.FunctionNames[ast.FuncNameCatalog] = "currentDatabase"
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

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	db, err := d.doOpen(ctx, src)
	if err != nil {
		return nil, err
	}

	if err = driver.OpeningPing(ctx, src, db); err != nil {
		return nil, err
	}

	return &grip{log: d.log, db: db, src: src, drvr: d}, nil
}

func (d *driveri) doOpen(ctx context.Context, src *source.Source) (*sql.DB, error) {
	ctx = options.NewContext(ctx, src.Options)

	// Parse the DSN to get connection details
	// Expected format: clickhouse://user:password@host:port/database
	db, err := sql.Open("clickhouse", src.Location)
	if err != nil {
		return nil, errw(err)
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

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	props := make(map[string]any)

	// Get ClickHouse version
	var version string
	err := db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return nil, errw(err)
	}
	props["version"] = version

	// Get current database
	var database string
	err = db.QueryRowContext(ctx, "SELECT currentDatabase()").Scan(&database)
	if err != nil {
		return nil, errw(err)
	}
	props["database"] = database

	return props, nil
}

// Truncate implements driver.Driver.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, _ bool) (affected int64, err error) {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return 0, errw(err)
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseDB, db)

	// Get row count before truncate
	affectedQuery := "SELECT COUNT(*) FROM " + stringz.BacktickQuote(tbl)
	err = db.QueryRowContext(ctx, affectedQuery).Scan(&affected)
	if err != nil {
		return 0, errw(err)
	}

	// ClickHouse uses TRUNCATE TABLE syntax
	truncateQuery := "TRUNCATE TABLE " + stringz.BacktickQuote(tbl)
	_, err = db.ExecContext(ctx, truncateQuery)
	if err != nil {
		return 0, errw(err)
	}

	return affected, nil
}

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	// ClickHouse uses databases instead of schemas
	stmt := "CREATE DATABASE IF NOT EXISTS " + stringz.BacktickQuote(schemaName)
	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	stmt := "DROP DATABASE IF EXISTS " + stringz.BacktickQuote(schemaName)
	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error {
	stmt := buildCreateTableStmt(tblDef)
	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	const query = `SELECT COUNT(*) FROM system.tables WHERE name = ? AND database = currentDatabase()`

	var count int64
	err := db.QueryRowContext(ctx, query, tbl).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count > 0, nil
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(ctx context.Context, db sqlz.DB, catalog string) (bool, error) {
	if catalog == "" {
		return false, nil
	}

	const query = `SELECT COUNT(*) FROM system.databases WHERE name = ?`

	var count int64
	err := db.QueryRowContext(ctx, query, catalog).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count > 0, nil
}

// CurrentCatalog implements driver.SQLDriver.
func (d *driveri) CurrentCatalog(ctx context.Context, db sqlz.DB) (string, error) {
	var catalog string
	const query = `SELECT currentDatabase()`

	if err := db.QueryRowContext(ctx, query).Scan(&catalog); err != nil {
		return "", errw(err)
	}
	return catalog, nil
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var schemaName string
	const query = `SELECT currentDatabase()`

	if err := db.QueryRowContext(ctx, query).Scan(&schemaName); err != nil {
		return "", errw(err)
	}
	return schemaName, nil
}

// ListCatalogs implements driver.SQLDriver.
func (d *driveri) ListCatalogs(ctx context.Context, db sqlz.DB) ([]string, error) {
	const query = `SELECT name FROM system.databases ORDER BY name`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var catalogs []string
	for rows.Next() {
		var catalog string
		if err = rows.Scan(&catalog); err != nil {
			return nil, errw(err)
		}
		catalogs = append(catalogs, catalog)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return catalogs, nil
}

// ListSchemaMetadata implements driver.SQLDriver.
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	log := lg.FromContext(ctx)
	const query = `SELECT name FROM system.databases ORDER BY name`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var schemas []*metadata.Schema
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return nil, errw(err)
		}

		s := &metadata.Schema{
			Name:    name,
			Catalog: name, // In ClickHouse, catalog and schema are the same (database)
		}
		schemas = append(schemas, s)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return schemas, nil
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)
	const query = `SELECT name FROM system.databases ORDER BY name`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var schemas []string
	for rows.Next() {
		var schemaName string
		if err = rows.Scan(&schemaName); err != nil {
			return nil, errw(err)
		}
		schemas = append(schemas, schemaName)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return schemas, nil
}

// ListTableNames implements driver.SQLDriver.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, schma string, tables, views bool) ([]string, error) {
	if !tables && !views {
		return []string{}, nil
	}

	// Build query based on table/view filter
	q := "SELECT name FROM system.tables WHERE database = "
	var args []any
	if schma == "" {
		q += "currentDatabase()"
	} else {
		q += "?"
		args = append(args, schma)
	}

	// ClickHouse uses 'engine' field to distinguish views from tables
	// Views have engine = 'View' or 'MaterializedView'
	if tables && !views {
		q += " AND engine NOT IN ('View', 'MaterializedView')"
	} else if views && !tables {
		q += " AND (engine = 'View' OR engine = 'MaterializedView')"
	}
	// If both tables and views are true, no filter needed

	q += " ORDER BY name"

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, errw(err)
	}

	names, err := sqlz.RowsScanColumn[string](ctx, rows)
	if err != nil {
		return nil, errw(err)
	}

	return names, nil
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error) {
	if schma == "" {
		return false, nil
	}

	const query = `SELECT COUNT(*) FROM system.databases WHERE name = ?`
	var count int64
	err := db.QueryRowContext(ctx, query, schma).Scan(&count)
	if err != nil {
		return false, errw(err)
	}

	return count > 0, nil
}

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(
	ctx context.Context, db sqlz.DB, fromTable, toTable tablefq.T, copyData bool,
) (int64, error) {
	// First, get the schema of the source table
	// Type assert sqlz.DB to *sql.DB for metadata functions
	sqlDB, ok := db.(*sql.DB)
	if !ok {
		return 0, errz.New("expected *sql.DB")
	}

	srcTbl, err := getTableMetadata(ctx, sqlDB, "", fromTable.Table)
	if err != nil {
		return 0, err
	}

	// Create the destination table with same schema
	destTblDef := &schema.Table{
		Name: toTable.Table,
		Cols: make([]*schema.Column, len(srcTbl.Columns)),
	}

	for i, col := range srcTbl.Columns {
		destTblDef.Cols[i] = &schema.Column{
			Name: col.Name,
			Kind: col.Kind,
		}
	}

	if err = d.CreateTable(ctx, db, destTblDef); err != nil {
		return 0, err
	}

	if !copyData {
		return 0, nil
	}

	// Copy data using INSERT INTO ... SELECT
	query := fmt.Sprintf("INSERT INTO %s SELECT * FROM %s",
		stringz.BacktickQuote(toTable.Table),
		stringz.BacktickQuote(fromTable.Table))

	result, err := db.ExecContext(ctx, query)
	if err != nil {
		return 0, errw(err)
	}

	affected, err := result.RowsAffected()
	return affected, errw(err)
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(
	ctx context.Context, db sqlz.DB, tblName string, colNames []string,
) ([]*sql.ColumnType, error) {
	const queryTpl = "SELECT %s FROM %s LIMIT 0"

	enquote := d.Dialect().Enquote
	tblNameQuoted := enquote(tblName)

	colsClause := "*"
	if len(colNames) > 0 {
		var quotedCols []string
		for _, col := range colNames {
			quotedCols = append(quotedCols, enquote(col))
		}
		colsClause = strings.Join(quotedCols, driver.Comma)
	}

	query := fmt.Sprintf(queryTpl, colsClause, tblNameQuoted)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)

	colTypes, err := rows.ColumnTypes()
	return colTypes, errw(err)
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

	query := buildUpdateStmt(destTbl, destColNames, where)

	stmt, err := db.PrepareContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}

	execer := driver.NewStmtExecer(stmt, driver.DefaultInsertMungeFunc(destTbl, destColsMeta), newStmtExecFunc(stmt),
		destColsMeta)
	return execer, nil
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		stringz.BacktickQuote(tbl),
		stringz.BacktickQuote(col),
		dbTypeNameFromKind(knd),
	)

	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf("RENAME TABLE %s TO %s",
		stringz.BacktickQuote(tbl),
		stringz.BacktickQuote(newName),
	)

	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	q := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
		stringz.BacktickQuote(tbl),
		stringz.BacktickQuote(col),
		stringz.BacktickQuote(newName),
	)

	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error {
	var stmt string
	if ifExists {
		stmt = "DROP TABLE IF EXISTS " + stringz.BacktickQuote(tbl.Table)
	} else {
		stmt = "DROP TABLE " + stringz.BacktickQuote(tbl.Table)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// AlterTableColumnKinds is not yet implemented for ClickHouse.
func (d *driveri) AlterTableColumnKinds(_ context.Context, _ sqlz.DB, _ string, _ []string, _ []kind.Kind) error {
	return errz.New("not implemented")
}

// grip implements driver.Grip.
type grip struct {
	log  *slog.Logger
	db   *sql.DB
	src  *source.Source
	drvr *driveri
}

// DB implements driver.Grip.
func (g *grip) DB(_ context.Context) (*sql.DB, error) {
	return g.db, nil
}

// SQLDriver implements driver.Grip.
func (g *grip) SQLDriver() driver.SQLDriver {
	return g.drvr
}

// Source implements driver.Grip.
func (g *grip) Source() *source.Source {
	return g.src
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	return getSourceMetadata(ctx, g.src, g.db, noSchema)
}

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	return getTableMetadata(ctx, g.db, "", tblName)
}

// Close implements driver.Grip.
func (g *grip) Close() error {
	return errz.Err(g.db.Close())
}

// getTableRecordMeta returns record metadata for specified columns in a table.
func (d *driveri) getTableRecordMeta(ctx context.Context, db sqlz.DB, tblName string,
	colNames []string,
) (record.Meta, error) {
	colTypes, err := d.TableColumnTypes(ctx, db, tblName, colNames)
	if err != nil {
		return nil, err
	}

	return recordMetaFromColumnTypes(ctx, colTypes)
}

// newStmtExecFunc returns a StmtExecFunc for executing prepared statements.
func newStmtExecFunc(stmt *sql.Stmt) driver.StmtExecFunc {
	return func(ctx context.Context, args ...any) (int64, error) {
		res, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return 0, errw(err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return 0, errw(err)
		}

		return affected, nil
	}
}

// errw wraps any error from the ClickHouse driver.
func errw(err error) error {
	if err == nil {
		return nil
	}
	return errz.Wrapf(err, "clickhouse")
}
