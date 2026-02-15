// Package clickhouse implements the sq driver for ClickHouse, a column-oriented
// OLAP database management system. This driver uses the clickhouse-go v2 library
// to communicate with ClickHouse servers via the native TCP protocol.
//
// # Connection String Format
//
// ClickHouse connection strings follow the URL format:
//
//	clickhouse://[username:password@]host[:port]/database[?param=value]
//
// Examples:
//
//	clickhouse://default:@localhost:9000/default
//	clickhouse://user:pass@host:9000/mydb?secure=true
//
// Unlike some database drivers (e.g., pgx for PostgreSQL), clickhouse-go does
// not automatically apply a default port. This driver handles that by applying
// port 9000 for non-secure connections or 9440 for secure connections (when
// secure=true is specified).
//
// # ClickHouse-Specific Behavior
//
// ClickHouse differs from traditional SQL databases in several ways that affect
// this driver's implementation:
//
//   - No ACID Transactions: ClickHouse is optimized for OLAP workloads and does
//     not support traditional transactions. Inserts are atomic at the batch level.
//
//   - MergeTree Engine: All tables created by this driver use the MergeTree
//     engine, which requires an ORDER BY clause. The first column is used as
//     the ordering key by default.
//
//   - ALTER TABLE UPDATE: ClickHouse does not support standard UPDATE statements.
//     Instead, this driver uses ALTER TABLE ... UPDATE syntax.
//
//   - Schema = Database: ClickHouse uses "database" terminology where other SQL
//     databases use "schema". This driver maps both concepts to ClickHouse databases.
//
//   - Type System: ClickHouse has distinct signed (Int8-64) and unsigned (UInt8-64)
//     integer types, nullable wrappers (Nullable(T)), and storage optimizations
//     (LowCardinality(T)). See the metadata.go file for type mapping details.
//
// # Driver Registration
//
// The driver is registered with sq's driver registry via the Provider type:
//
//	registry.AddProvider(drivertype.ClickHouse, &clickhouse.Provider{Log: log})
package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
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

	// defaultPort is the default ClickHouse native protocol port (non-secure).
	defaultPort = 9000

	// defaultSecurePort is the default ClickHouse native protocol port with TLS.
	defaultSecurePort = 9440
)

// locationWithDefaultPort returns the location string with the default port
// added if no port is specified. The second return value is true if the port
// was added. Unlike some other database drivers (e.g. pgx for Postgres),
// clickhouse-go does not apply a default port automatically.
//
// The default port depends on whether TLS is enabled:
//   - Non-secure (default): 9000
//   - Secure (secure=true): 9440
func locationWithDefaultPort(loc string) (string, bool, error) {
	u, err := url.Parse(loc)
	if err != nil {
		return "", false, errz.Wrapf(err, "parse clickhouse location")
	}

	if u.Port() != "" {
		// Port already specified, return as-is
		return loc, false, nil
	}

	// Determine the appropriate default port based on secure parameter
	port := defaultPort
	if u.Query().Get("secure") == "true" {
		port = defaultSecurePort
	}

	// No port specified, add the default port
	u.Host = u.Hostname() + ":" + strconv.Itoa(port)
	return u.String(), true, nil
}

// Provider is the ClickHouse implementation of driver.Provider.
// It serves as a factory for creating ClickHouse driver instances.
//
// Provider is registered with sq's driver registry to handle sources
// with the "clickhouse" driver type. When sq needs to interact with a
// ClickHouse source, it calls DriverFor to obtain a driver instance.
type Provider struct {
	// Log is the logger used by driver instances created by this provider.
	// It should be set before registering the provider with the driver registry.
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

// driveri is the ClickHouse implementation of driver.SQLDriver.
// It provides all the database operations needed by sq to interact with
// ClickHouse sources, including connection management, DDL operations
// (CREATE TABLE, DROP TABLE, etc.), DML operations (INSERT, UPDATE),
// and metadata retrieval.
//
// The "i" suffix follows the sq convention for internal driver implementations
// that are not exported (e.g., driveri, grip).
type driveri struct {
	// log is the logger for driver operations. It is passed from the Provider
	// when the driver is created via DriverFor.
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver. It returns the ClickHouse-specific
// connection parameters that can be used in connection strings. Each parameter
// maps to a list of valid values (or example values for open-ended parameters).
//
// Supported parameters:
//   - dial_timeout: Connection timeout (e.g., "10s")
//   - compress: Compression algorithm (true, false, lz4, zstd, gzip)
//   - secure: Enable TLS (true, false)
//   - skip_verify: Skip TLS certificate verification (true, false)
//   - connection_open_strategy: Load balancing strategy (in_order, round_robin)
//   - max_open_conns: Maximum open connections
//   - max_idle_conns: Maximum idle connections
//   - conn_max_lifetime: Connection maximum lifetime (e.g., "1h")
func (d *driveri) ConnParams() map[string][]string {
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

// Dialect implements driver.SQLDriver. It returns the SQL dialect configuration
// for ClickHouse, which defines how SQL is generated for this database.
//
// Key dialect characteristics:
//   - Placeholders: Uses ? for positional parameters (like MySQL)
//   - Enquote: Uses backticks for identifier quoting (e.g., `table_name`)
//   - MaxBatchValues: 10000 (ClickHouse is optimized for large batch inserts)
//   - Joins: Supports all standard join types
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:                      Type,
		Placeholders:              placeholders,
		Enquote:                   stringz.BacktickQuote,
		ExecModeFor:               dialect.DefaultExecModeFor,
		MaxBatchValues:            10000,
		Ops:                       dialect.DefaultOps(),
		Joins:                     jointype.All(),
		IsRowsAffectedUnsupported: true,
	}
}

// placeholders generates SQL placeholder strings for parameterized queries.
// ClickHouse uses positional ? placeholders (like MySQL), not numbered
// placeholders (like PostgreSQL's $1, $2).
//
// For example, placeholders(2, 3) returns "(?, ?), (?, ?), (?, ?)".
//
// This function is used by the dialect to generate INSERT statements with
// multiple rows of values.
func placeholders(numCols, numRows int) string {
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

// Renderer implements driver.SQLDriver. It returns a SQL renderer configured
// for ClickHouse's SQL dialect.
//
// The renderer maps sq's built-in functions to ClickHouse equivalents:
//   - schema() -> currentDatabase()
//   - catalog() -> currentDatabase()
//
// Both schema and catalog map to currentDatabase() because ClickHouse uses
// "database" terminology where other SQL databases distinguish between
// catalogs and schemas.
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

// Open implements driver.Driver. It opens a connection to the ClickHouse
// source and returns a Grip (database handle) for performing operations.
//
// The connection process:
//  1. Opens a database connection using doOpen
//  2. Performs a ping to verify connectivity
//  3. Returns a grip wrapping the connection
//
// The returned grip should be closed when no longer needed to release
// the database connection.
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

// doOpen creates the underlying sql.DB connection to ClickHouse.
// It handles default port application and connection pool configuration.
//
// This is an internal helper used by both Open (for normal connections)
// and other methods that need temporary connections (e.g., Ping, Truncate).
func (d *driveri) doOpen(ctx context.Context, src *source.Source) (*sql.DB, error) {
	ctx = options.NewContext(ctx, src.Options)
	log := lg.FromContext(ctx)

	// Apply default port if not specified. This is a fallback for legacy sources
	// that may have been added before ValidateSource applied the default port.
	// Unlike some other database drivers (e.g. pgx for Postgres), clickhouse-go
	// does not apply a default port automatically.
	loc, portAdded, err := locationWithDefaultPort(src.Location)
	if err != nil {
		return nil, err
	}

	if portAdded {
		log.Debug("Applied default ClickHouse port at connection time",
			lga.Src, src.Handle,
			lga.Before, src.Location,
			lga.After, loc,
			lga.Default, defaultPort,
		)
	}

	db, err := sql.Open("clickhouse", loc)
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

	// Apply default port if not specified. Unlike some other database drivers
	// (e.g. pgx for Postgres), clickhouse-go does not apply a default port.
	loc, portAdded, err := locationWithDefaultPort(src.Location)
	if err != nil {
		return nil, errw(err)
	}

	if portAdded {
		d.log.Debug("Applied default ClickHouse port to source location",
			lga.Src, src.Handle,
			lga.Before, src.Location,
			lga.After, loc,
			lga.Default, defaultPort,
		)
		src = src.Clone()
		src.Location = loc
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

// Truncate implements driver.Driver. It removes all rows from the specified
// table and returns the number of rows that were deleted.
//
// Note: The second parameter (cascade) is ignored for ClickHouse as it does
// not support cascading truncates in the same way as other databases.
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

// CreateSchema implements driver.SQLDriver. It creates a new database in
// ClickHouse.
//
// Note: ClickHouse uses "database" terminology where other SQL databases use
// "schema". This method creates a ClickHouse database using CREATE DATABASE
// IF NOT EXISTS syntax.
func (d *driveri) CreateSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	stmt := "CREATE DATABASE IF NOT EXISTS " + stringz.BacktickQuote(schemaName)
	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// DropSchema implements driver.SQLDriver. It drops a database from ClickHouse.
//
// This uses DROP DATABASE IF EXISTS syntax, so it will not error if the
// database does not exist.
func (d *driveri) DropSchema(ctx context.Context, db sqlz.DB, schemaName string) error {
	stmt := "DROP DATABASE IF EXISTS " + stringz.BacktickQuote(schemaName)
	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// CreateTable implements driver.SQLDriver. It creates a new table in ClickHouse
// using the MergeTree engine.
//
// The table is created with:
//   - MergeTree engine (required for most ClickHouse operations)
//   - ORDER BY clause using the first column (required by MergeTree)
//   - Nullable wrappers for columns where NotNull is false
//
// See buildCreateTableStmt in render.go for the full CREATE TABLE generation.
func (d *driveri) CreateTable(ctx context.Context, db sqlz.DB, tblDef *schema.Table) error {
	stmt := buildCreateTableStmt(tblDef)
	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// TableExists implements driver.SQLDriver. It checks if a table exists in the
// current database by querying system.tables.
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

// ListTableNames implements driver.SQLDriver. It returns the names of tables
// and/or views in the specified schema (database).
//
// Parameters:
//   - schma: The database name. If empty, uses the current database.
//   - tables: Include regular tables in the result.
//   - views: Include views (View and MaterializedView engines) in the result.
//
// ClickHouse distinguishes tables from views via the engine field in
// system.tables. Views have engine "View" or "MaterializedView"; all other
// engines are considered tables.
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

	// ClickHouse uses 'engine' field to distinguish views from tables.
	// Views have engine = 'View' or 'MaterializedView'.
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

// CopyTable implements driver.SQLDriver. It creates a copy of a table,
// optionally including its data.
//
// The process:
//  1. Retrieves the source table's schema (column names and types)
//     via [getTableMetadata], which queries ClickHouse system.columns.
//  2. Creates the destination table with the same column schema using
//     [driveri.CreateTable] (MergeTree engine, ORDER BY first NOT NULL
//     column or tuple()).
//  3. If copyData is true, copies all rows using INSERT INTO ... SELECT.
//
// Both fromTable and toTable are rendered via [tblfmt], which preserves
// any schema or catalog qualifiers present in the [tablefq.T] arguments.
//
// Return values:
//   - If copyData is false, returns (0, nil) — no rows copied.
//   - If copyData is true, returns ([dialect.RowsAffectedUnsupported], nil)
//     on success because ClickHouse does not report row counts for
//     INSERT ... SELECT operations (RowsAffected() always returns 0).
//     Callers that need the actual count must issue a separate
//     SELECT COUNT(*) query.
//
// Note: The destination table always uses the MergeTree engine with
// the first NOT NULL column as the ORDER BY key. This may differ from
// the source table's engine and ordering configuration.
func (d *driveri) CopyTable(
	ctx context.Context, db sqlz.DB, fromTable, toTable tablefq.T, copyData bool,
) (int64, error) {
	// First, get the schema of the source table.
	srcTbl, err := getTableMetadata(ctx, db, "", fromTable.Table)
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

	// Copy data using INSERT INTO ... SELECT.
	//
	// ClickHouse Limitation: ClickHouse does not report row counts for
	// INSERT ... SELECT operations. The RowsAffected() method on sql.Result
	// returns 0 regardless of how many rows were actually copied. This is a
	// fundamental limitation of ClickHouse's protocol, not a driver bug.
	//
	// To handle this, we return dialect.RowsAffectedUnsupported (-1) to signal
	// to callers that the row count is unavailable. Callers (e.g., CLI, tests)
	// should check for this value and either:
	//   - Display an "unavailable" message to users
	//   - Verify success via an explicit row count query if needed
	//
	// See: drivers/clickhouse/README.md "Known Limitations" section.
	query := fmt.Sprintf("INSERT INTO %s SELECT * FROM %s",
		tblfmt(toTable),
		tblfmt(fromTable))

	_, err = db.ExecContext(ctx, query)
	if err != nil {
		return 0, errw(err)
	}

	return dialect.RowsAffectedUnsupported, nil
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

// PrepareInsertStmt implements driver.SQLDriver. It prepares a batch INSERT
// statement for inserting multiple rows into a table.
//
// The returned StmtExecer handles value transformation (munging) to ensure
// values are in the correct format for ClickHouse, and provides an execution
// function for running the prepared statement.
//
// Parameters:
//   - destTbl: The target table name
//   - destColNames: Columns to insert into
//   - numRows: Number of rows to insert per batch
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

// PrepareUpdateStmt implements driver.SQLDriver. It prepares an UPDATE statement
// for modifying rows in a table.
//
// Note: ClickHouse does not support standard SQL UPDATE statements. Instead,
// this uses ALTER TABLE ... UPDATE syntax, which is ClickHouse's mechanism for
// row-level updates.
//
// ClickHouse mutations are asynchronous by default: the ALTER TABLE UPDATE
// statement returns immediately, before the data is actually modified. A
// subsequent SELECT may still return stale (pre-mutation) data. To ensure
// the mutation completes before this method returns, the query appends
// SETTINGS mutations_sync = 1, which forces synchronous execution.
//
// Even with mutations_sync = 1, ClickHouse does not report the number of
// rows affected by a mutation. The sql.Result.RowsAffected() value returned
// by the driver is always 0. The returned StmtExecer therefore returns
// [dialect.RowsAffectedUnsupported] (-1) to signal "unknown" rather than
// a misleading 0.
//
// Implementation detail: clickhouse-go v2's PrepareContext() only supports
// INSERT and SELECT statements. It internally classifies every non-SELECT
// statement as INSERT and validates accordingly, rejecting ALTER TABLE
// UPDATE with "invalid INSERT query".
// See https://github.com/ClickHouse/clickhouse-go/issues/1203.
//
// To work around this, PrepareUpdateStmt bypasses PrepareContext()
// entirely: it builds the ALTER TABLE UPDATE query string via
// [buildUpdateStmt], appends "SETTINGS mutations_sync = 1" (so the
// mutation completes synchronously), and wraps a direct ExecContext()
// call in a [driver.StmtExecFunc] closure. A nil stmt is passed to
// [driver.NewStmtExecer]; the StmtExecer.Close() method is nil-safe
// to support this pattern.
//
// The returned [driver.StmtExecer] always reports
// [dialect.RowsAffectedUnsupported] (-1) for affected rows because
// ClickHouse's RowsAffected() returns 0 regardless of how many rows
// were actually modified by a mutation.
//
// Parameters:
//   - destTbl: the target table name.
//   - destColNames: columns to update; must be non-empty (validated
//     by [buildUpdateStmt]).
//   - where: WHERE clause body without the "WHERE" keyword. Pass ""
//     to update all rows.
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

	// ClickHouse mutations (ALTER TABLE UPDATE) are asynchronous by default:
	// the statement returns immediately, before the data is actually modified.
	// Without mutations_sync=1, a subsequent SELECT may return stale
	// (pre-mutation) data. Append SETTINGS mutations_sync=1 so the mutation
	// completes synchronously before ExecContext returns.
	query += " SETTINGS mutations_sync = 1"

	// clickhouse-go's PrepareContext only supports INSERT/SELECT,
	// rejecting ALTER TABLE UPDATE with "invalid INSERT query".
	// See https://github.com/ClickHouse/clickhouse-go/issues/1203
	// Bypass PrepareContext and use ExecContext directly.
	execFn := func(ctx context.Context, args ...any) (int64, error) {
		_, err := db.ExecContext(ctx, query, args...)
		if err != nil {
			return 0, errw(err)
		}
		// ClickHouse does not report rows affected for mutations:
		// RowsAffected() always returns 0 regardless of how many rows
		// were actually modified. Return RowsAffectedUnsupported (-1)
		// to signal "unknown" rather than a misleading 0.
		return dialect.RowsAffectedUnsupported, nil
	}

	// Pass nil for stmt because we bypass PrepareContext() (see above).
	// StmtExecer.Close() is nil-safe to support this.
	execer := driver.NewStmtExecer(nil,
		driver.DefaultInsertMungeFunc(destTbl, destColsMeta),
		execFn, destColsMeta)
	return execer, nil
}

// AlterTableAddColumn implements driver.SQLDriver. It adds a new column to an
// existing table using ALTER TABLE ... ADD COLUMN syntax.
//
// The column type is wrapped with Nullable() because sq defaults to nullable
// columns (NotNull is false by default), but ClickHouse columns are
// non-nullable by default. This matches the behavior of buildCreateTableStmt.
func (d *driveri) AlterTableAddColumn(ctx context.Context, db sqlz.DB, tbl, col string, knd kind.Kind) error {
	q := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s Nullable(%s)",
		stringz.BacktickQuote(tbl),
		stringz.BacktickQuote(col),
		dbTypeNameFromKind(knd),
	)

	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// AlterTableRename implements driver.SQLDriver. It renames a table using
// RENAME TABLE syntax.
func (d *driveri) AlterTableRename(ctx context.Context, db sqlz.DB, tbl, newName string) error {
	q := fmt.Sprintf("RENAME TABLE %s TO %s",
		stringz.BacktickQuote(tbl),
		stringz.BacktickQuote(newName),
	)

	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// AlterTableRenameColumn implements driver.SQLDriver. It renames a column
// using ALTER TABLE ... RENAME COLUMN syntax.
func (d *driveri) AlterTableRenameColumn(ctx context.Context, db sqlz.DB, tbl, col, newName string) error {
	q := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
		stringz.BacktickQuote(tbl),
		stringz.BacktickQuote(col),
		stringz.BacktickQuote(newName),
	)

	_, err := db.ExecContext(ctx, q)
	return errw(err)
}

// DropTable implements driver.SQLDriver. It drops a table from the
// database using ClickHouse's DROP TABLE statement.
//
// The table reference is rendered via [tblfmt], which preserves any
// schema or catalog qualifiers present in the [tablefq.T] argument.
// For example, a tablefq.T with Schema="mydb" and Table="actors"
// produces DROP TABLE `mydb`.`actors`.
//
// If ifExists is true, the statement uses DROP TABLE IF EXISTS so
// that dropping a non-existent table does not return an error. This
// is used by cleanup paths (e.g. deferred drops in tests) where the
// table may or may not exist.
func (d *driveri) DropTable(ctx context.Context, db sqlz.DB, tbl tablefq.T, ifExists bool) error {
	var stmt string
	if ifExists {
		stmt = "DROP TABLE IF EXISTS " + tblfmt(tbl)
	} else {
		stmt = "DROP TABLE " + tblfmt(tbl)
	}

	_, err := db.ExecContext(ctx, stmt)
	return errw(err)
}

// AlterTableColumnKinds implements driver.SQLDriver. It changes the types of
// the specified columns using ALTER TABLE ... MODIFY COLUMN.
func (d *driveri) AlterTableColumnKinds(ctx context.Context,
	db sqlz.DB, tbl string, colNames []string, kinds []kind.Kind,
) error {
	if len(colNames) != len(kinds) {
		return errz.Errorf(
			"clickhouse: alter table column kinds: mismatched count"+
				" of columns (%d) and kinds (%d)",
			len(colNames), len(kinds),
		)
	}

	for i, col := range colNames {
		q := fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s Nullable(%s)",
			stringz.BacktickQuote(tbl),
			stringz.BacktickQuote(col),
			dbTypeNameFromKind(kinds[i]),
		)

		if _, err := db.ExecContext(ctx, q); err != nil {
			return errw(err)
		}
	}

	return nil
}

// getTableRecordMeta returns record metadata for specified columns in a table.
// It queries the table with LIMIT 0 to get column type information, then
// builds record.Meta with appropriate scan types for each column.
//
// This is used by PrepareInsertStmt and PrepareUpdateStmt to determine
// how values should be transformed before being passed to ClickHouse.
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
// The returned function executes the statement with the given arguments and
// returns [dialect.RowsAffectedUnsupported] because ClickHouse does not
// reliably report rows affected for INSERT operations.
//
// This is used as part of the StmtExecer returned by PrepareInsertStmt
// to provide the actual execution logic.
func newStmtExecFunc(stmt *sql.Stmt) driver.StmtExecFunc {
	return func(ctx context.Context, args ...any) (int64, error) {
		_, err := stmt.ExecContext(ctx, args...)
		if err != nil {
			return 0, errw(err)
		}

		return dialect.RowsAffectedUnsupported, nil
	}
}
