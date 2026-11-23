// Package databricks implements the sq driver for Databricks SQL warehouses.
// It uses the official database/sql driver, databricks-sql-go, which talks to
// the warehouse's HTTP endpoint (the Thrift-over-HTTP protocol).
//
// # Location format
//
//	databricks://token:PAT@HOST[:PORT]/HTTP_PATH[?PARAM=VAL...]
//
// Examples:
//
//	databricks://token:dapi123@dbc-a1b2.cloud.databricks.com/sql/1.0/warehouses/abc123
//	databricks://token:dapi123@dbc-a1b2.cloud.databricks.com/sql/1.0/warehouses/abc123?catalog=main&schema=sakila
//	databricks://dbc-a1b2.cloud.databricks.com/sql/1.0/warehouses/abc123?clientID=ID&clientSecret=SECRET
//
// The username is always the literal "token", and the password is a personal
// access token (PAT). Without userinfo, OAuth machine-to-machine (M2M) is
// selected by the clientID and clientSecret query params. HTTP_PATH may
// instead be given as the http_path query param. See buildDSN for details.
//
// The source's Catalog and Schema fields, which the --src.schema flag sets,
// override the catalog and schema params of the location.
//
// # Scope
//
// This driver is read-only. It supports connecting, pinging, SLQ and native SQL
// queries, joins against other sources, and inspecting metadata. It does not
// create, alter, copy, truncate or drop tables, and it does not insert rows:
// those SQLDriver methods return an error. Native SQL passed to "sq sql" is
// not restricted, so DDL and DML can still be run that way.
//
// # Databricks-specific behavior
//
//   - Catalog and schema: Databricks (Unity Catalog) has a three-level
//     namespace, catalog.schema.table. The dialect sets Catalog to true. The
//     current catalog and schema are those of the connection: the catalog and
//     schema query params if present, else the workspace defaults.
//
//   - Metadata comes from the information_schema of the current catalog, which
//     only exists for Unity Catalog. The legacy hive_metastore catalog has no
//     information_schema, so inspecting it is not supported. Queries against
//     it still work.
//
//   - Constraints: primary keys are reported, but Databricks does not enforce
//     them (or foreign keys). They are informational only.
//
//   - Complex types (ARRAY, MAP, STRUCT, VARIANT) are returned by the
//     underlying driver as JSON strings, and map to kind.Text.
//
//   - Rows affected: the count is reported as unsupported. See
//     dialect.IsRowsAffectedUnsupported.
//
//   - Transactions are not supported by Databricks, and sq does not use them
//     for reading.
//
// # Driver Registration
//
// The driver is registered with sq's driver registry via the Provider type:
//
//	registry.AddProvider(drivertype.Databricks, &databricks.Provider{Log: log})
package databricks

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"

	_ "github.com/databricks/databricks-sql-go" // Databricks database/sql driver.
	dbsqllogger "github.com/databricks/databricks-sql-go/logger"

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
	"github.com/neilotoole/sq/libsq/core/secret"
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
	// Type is the Databricks driver type.
	Type = drivertype.Databricks

	// defaultPort is the default port of a Databricks workspace (HTTPS).
	defaultPort = 443

	// maxBatchValues would be the number of values in a batch insert. It is
	// not used, as the driver is read-only, but the dialect must declare a
	// positive value. It's deliberately conservative.
	maxBatchValues = 100
)

// Provider is the Databricks implementation of driver.Provider.
type Provider struct {
	// Log is the logger used by driver instances created by this provider.
	Log *slog.Logger
}

// logSinkOnce guards the one-time redirection of the log output of
// databricks-sql-go, which is process-global.
var logSinkOnce sync.Once

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	if p.Log != nil {
		logSinkOnce.Do(func() { redirectDriverLog(p.Log) })
	}

	return &driveri{log: p.Log}, nil
}

// logWriter is an io.Writer that forwards each write, which databricks-sql-go
// makes one log event at a time, to a slog.Logger at debug level.
type logWriter struct {
	log *slog.Logger
}

// Write implements io.Writer.
func (w logWriter) Write(p []byte) (int, error) {
	w.log.Debug("databricks-sql-go", "msg", string(bytes.TrimSpace(p)))
	return len(p), nil
}

// redirectDriverLog sends the log output of databricks-sql-go to log. By
// default that library writes warnings to stderr, which would interleave with
// sq's own output. If the user has set DATABRICKS_LOG_LEVEL they have asked
// for the library's logging, and it is left alone.
func redirectDriverLog(log *slog.Logger) {
	if _, ok := os.LookupEnv("DATABRICKS_LOG_LEVEL"); ok {
		return
	}
	dbsqllogger.SetLogOutput(logWriter{log: log})
}

var (
	_ driver.Provider  = (*Provider)(nil)
	_ driver.SQLDriver = (*driveri)(nil)
)

// driveri is the Databricks implementation of driver.SQLDriver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver. The params are those of the sq
// location, most of which are interpreted by databricks-sql-go.
func (d *driveri) ConnParams() map[string][]string {
	return map[string][]string{
		"catalog":            {"main"},
		"schema":             {"default"},
		"http_path":          {"/sql/1.0/warehouses/"},
		"authType":           {"Pat", "OauthM2M"},
		"clientID":           nil,
		"clientSecret":       nil,
		"maxRows":            {"100000"},
		"timeout":            {"300"},
		"useCloudFetch":      {"true", "false"},
		"maxDownloadThreads": {"10"},
		"enableTelemetry":    {"true", "false"},
		"timezone":           {"UTC"},
	}
}

// LocationShape implements driver.SQLDriver.
func (d *driveri) LocationShape() driver.LocationShape {
	return driver.LocationShape{
		Type:    Type,
		Schemes: []string{"databricks"},
		Segments: []driver.Segment{
			{Kind: driver.SegCredentials, Optional: true},
			{Kind: driver.SegAuthority},
			{Kind: driver.SegPathName, Optional: true, Placeholder: "http_path"},
			{Kind: driver.SegConnParams, Optional: true},
		},
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
		Description: "Databricks",
		Doc:         "https://github.com/databricks/databricks-sql-go",
		IsSQL:       true,
		DefaultPort: defaultPort,
	}
}

// Dialect implements driver.SQLDriver.
//
// Key dialect characteristics:
//   - Placeholders: positional "?".
//   - Enquote: backticks, as in MySQL.
//   - QuoteString: backslash escapes, because Spark SQL does not read a
//     doubled single quote as a quote. See quoteString.
//   - Catalog: true. Databricks has catalog.schema.table.
//   - IsRowsAffectedUnsupported: true. The count that databricks-sql-go reports
//     comes from the operation status, which was not verified to be populated
//     for DML on a live warehouse, so it is treated as unknown.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:                      Type,
		Placeholders:              placeholders,
		Enquote:                   stringz.BacktickQuote,
		QuoteString:               quoteString,
		ExecModeFor:               dialect.DefaultExecModeFor,
		MaxBatchValues:            maxBatchValues,
		Ops:                       dialect.DefaultOps(),
		Joins:                     jointype.All(),
		Catalog:                   true,
		IsRowsAffectedUnsupported: true,
	}
}

// placeholders returns the SQL placeholders for numRows rows of numCols
// columns, e.g. "(?, ?), (?, ?)".
func placeholders(numCols, numRows int) string {
	rows := make([]string, numRows)
	row := "(" + strings.TrimSuffix(strings.Repeat("?, ", numCols), ", ") + ")"
	for i := range rows {
		rows[i] = row
	}
	return strings.Join(rows, driver.Comma)
}

// Renderer implements driver.SQLDriver.
//
// The renderer maps sq's schema() and catalog() functions to current_schema()
// and current_catalog(), uses a Databricks-specific row range (see
// renderRange), and casts sum() to DECIMAL(38, 6) so that the result is a
// decimal for every input type (issue #839). Everything else, including
// LIKE ... ESCAPE, uses the default rendering, which Spark SQL accepts.
func (d *driveri) Renderer() *render.Renderer {
	r := render.NewDefaultRenderer()
	r.FunctionNames[ast.FuncNameSchema] = "current_schema"
	r.FunctionNames[ast.FuncNameCatalog] = "current_catalog"
	r.FunctionOverrides[ast.FuncNameSum] = render.FuncOverrideCastResult(
		fmt.Sprintf("DECIMAL(%d, %d)", render.AggDecimalPrecision, render.AggDecimalScale),
	)
	r.Range = renderRange
	return r
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(ctx context.Context, colTypes []*sql.ColumnType, _ map[int]kind.Kind) (
	record.Meta, driver.NewRecordFunc, error,
) {
	recMeta, err := recordMetaFromColumnTypes(ctx, colTypes)
	if err != nil {
		return nil, nil, err
	}
	return recMeta, getNewRecordFunc(recMeta), nil
}

// Open implements driver.Driver. The driver is read-only, and ignores the
// access mode.
func (d *driveri) Open(ctx context.Context, src *source.Source, _ driver.AccessMode) (driver.Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	db, err := d.doOpen(ctx, src)
	if err != nil {
		return nil, err
	}

	ver, err := driver.OpeningPing(ctx, src, db, d.DBSemver)
	if err != nil {
		return nil, err
	}

	g := &grip{log: d.log, db: db, src: src, drvr: d}
	g.semver.Prime(ver)
	return g, nil
}

// doOpen creates the sql.DB for src. It does not connect: the first
// connection is made lazily.
func (d *driveri) doOpen(ctx context.Context, src *source.Source) (*sql.DB, error) {
	ctx = options.NewContext(ctx, src.Options)

	dsn, err := buildDSN(src.Location, src.Catalog, src.Schema)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("databricks", dsn)
	if err != nil {
		return nil, errw(scrub(err, secretsOf(dsn)))
	}

	driver.ConfigureDB(ctx, db, src.Options)
	return db, nil
}

// ValidateSource implements driver.Driver. It checks that the location can
// be turned into a DSN, but does not change the source. A location that
// contains a ${scheme:path} placeholder can't be parsed until the placeholder
// is resolved, so it is not checked here: the same check runs at Open time.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != Type {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", Type, src.Type)
	}

	refs, err := secret.ExtractRefs(src.Location)
	if err != nil {
		return nil, errw(err)
	}
	if len(refs) > 0 {
		return src, nil
	}

	if _, err = buildDSN(src.Location, src.Catalog, src.Schema); err != nil {
		return nil, err
	}

	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source, _ driver.AccessMode) error {
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseDB, db)

	dsn, _ := buildDSN(src.Location, src.Catalog, src.Schema)
	if err = db.PingContext(ctx); err != nil {
		return errz.Wrapf(scrub(err, secretsOf(dsn)), "ping %s", src.Handle)
	}
	return nil
}

// CurrentCatalog implements driver.SQLDriver.
func (d *driveri) CurrentCatalog(ctx context.Context, db sqlz.DB) (string, error) {
	var catalog string
	if err := db.QueryRowContext(ctx, "SELECT current_catalog()").Scan(&catalog); err != nil {
		return "", errw(err)
	}
	return catalog, nil
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(ctx context.Context, db sqlz.DB) (string, error) {
	var schma string
	if err := db.QueryRowContext(ctx, "SELECT current_schema()").Scan(&schma); err != nil {
		return "", errw(err)
	}
	return schma, nil
}

// ListCatalogs implements driver.SQLDriver. The first element is the
// current catalog, and the remainder are sorted alphabetically.
func (d *driveri) ListCatalogs(ctx context.Context, db sqlz.DB) ([]string, error) {
	current, err := d.CurrentCatalog(ctx, db)
	if err != nil {
		return nil, err
	}

	names, err := queryNames(ctx, db, "SHOW CATALOGS")
	if err != nil {
		return nil, err
	}

	catalogs := make([]string, 0, len(names)+1)
	catalogs = append(catalogs, current)
	for _, name := range names {
		if name != current {
			catalogs = append(catalogs, name)
		}
	}
	sort.Strings(catalogs[1:])
	return catalogs, nil
}

// ListSchemas implements driver.SQLDriver. It lists the schemas of the
// current catalog.
func (d *driveri) ListSchemas(ctx context.Context, db sqlz.DB) ([]string, error) {
	names, err := queryNames(ctx, db, "SHOW SCHEMAS")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}

// ListSchemaMetadata implements driver.SQLDriver.
func (d *driveri) ListSchemaMetadata(ctx context.Context, db sqlz.DB) ([]*metadata.Schema, error) {
	catalog, err := d.CurrentCatalog(ctx, db)
	if err != nil {
		return nil, err
	}

	names, err := d.ListSchemas(ctx, db)
	if err != nil {
		return nil, err
	}

	schemas := make([]*metadata.Schema, len(names))
	for i, name := range names {
		schemas[i] = &metadata.Schema{Name: name, Catalog: catalog}
	}
	return schemas, nil
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(ctx context.Context, db sqlz.DB, catalog string) (bool, error) {
	if catalog == "" {
		return false, nil
	}

	names, err := queryNames(ctx, db, "SHOW CATALOGS")
	if err != nil {
		return false, err
	}
	return containsFold(names, catalog), nil
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(ctx context.Context, db sqlz.DB, schma string) (bool, error) {
	if schma == "" {
		return false, nil
	}

	names, err := queryNames(ctx, db, "SHOW SCHEMAS")
	if err != nil {
		return false, err
	}
	return containsFold(names, schma), nil
}

// TableExists implements driver.SQLDriver. It checks the current schema.
func (d *driveri) TableExists(ctx context.Context, db sqlz.DB, tbl string) (bool, error) {
	names, err := d.ListTableNames(ctx, db, "", true, true)
	if err != nil {
		return false, err
	}
	return containsFold(names, tbl), nil
}

// ListTableNames implements driver.SQLDriver. If schma is empty, the current
// schema is used. Tables and views are told apart by the information_schema
// table_type. It requires Unity Catalog, as it reads information_schema.
func (d *driveri) ListTableNames(ctx context.Context, db sqlz.DB, schma string, tables, views bool) ([]string, error) {
	if !tables && !views {
		return []string{}, nil
	}

	var catalog, curSchema string
	if err := db.QueryRowContext(ctx, "SELECT current_catalog(), current_schema()").
		Scan(&catalog, &curSchema); err != nil {
		return nil, errw(err)
	}
	if schma == "" {
		schma = curSchema
	}

	if err := requireInfoSchema(catalog); err != nil {
		return nil, err
	}

	q := "SELECT table_name FROM " + infoSchema(catalog) + ".tables WHERE table_schema = ?"
	switch {
	case tables && !views:
		q += " AND table_type NOT IN ('VIEW', 'MATERIALIZED_VIEW', 'METRIC_VIEW')"
	case views && !tables:
		q += " AND table_type IN ('VIEW', 'MATERIALIZED_VIEW', 'METRIC_VIEW')"
	}
	q += " ORDER BY table_name"

	rows, err := db.QueryContext(ctx, q, schma)
	if err != nil {
		return nil, errw(err)
	}
	return queryNamesFromRows(ctx, rows)
}

// TableColumnTypes implements driver.SQLDriver. Databricks identifiers are
// case-insensitive, so unlike drivers for case-sensitive engines, there is no
// need to fold colNames to the table's stored case.
func (d *driveri) TableColumnTypes(
	ctx context.Context, db sqlz.DB, tblName string, colNames []string,
) ([]*sql.ColumnType, error) {
	colsClause := "*"
	if len(colNames) > 0 {
		quoted := make([]string, len(colNames))
		for i, col := range colNames {
			quoted[i] = stringz.BacktickQuote(col)
		}
		colsClause = strings.Join(quoted, driver.Comma)
	}

	query := fmt.Sprintf("SELECT %s FROM %s LIMIT 0", colsClause, tblfmt(tblName))
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer lg.WarnIfFuncError(d.log, lgm.CloseDBRows, rows.Close)

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, errw(err)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return colTypes, nil
}

// requireInfoSchema returns an error if catalog is one of the legacy catalogs
// that has no information_schema.
func requireInfoSchema(catalog string) error {
	if strings.EqualFold(catalog, "hive_metastore") || strings.EqualFold(catalog, "spark_catalog") {
		return errz.Errorf("databricks: reading metadata of catalog %q is not supported: "+
			"it has no information_schema (only Unity Catalog catalogs do). "+
			"Queries against it work, and so does setting the catalog param to "+
			"a Unity Catalog catalog", catalog)
	}
	return nil
}

// containsFold returns true if names contains s, ignoring case: Databricks
// identifiers are case-insensitive.
func containsFold(names []string, s string) bool {
	return slices.ContainsFunc(names, func(name string) bool {
		return strings.EqualFold(name, s)
	})
}

// queryNames runs q, and returns the value of the first column of each row.
// It tolerates result sets with more columns, because the shape of SHOW
// statements differs a little between Databricks versions.
func queryNames(ctx context.Context, db sqlz.DB, q string) ([]string, error) {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}
	return queryNamesFromRows(ctx, rows)
}

// queryNamesFromRows reads rows, returning the value of the first column of
// each row. It closes rows.
func queryNamesFromRows(ctx context.Context, rows *sql.Rows) ([]string, error) {
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	cols, err := rows.Columns()
	if err != nil {
		return nil, errw(err)
	}
	if len(cols) == 0 {
		return nil, errz.New("databricks: result set has no columns")
	}

	names := []string{}
	for rows.Next() {
		dest := make([]any, len(cols))
		for i := range dest {
			dest[i] = new(any)
		}
		var name string
		dest[0] = &name
		if err = rows.Scan(dest...); err != nil {
			return nil, errw(err)
		}
		names = append(names, name)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return names, nil
}

// The methods below would modify the source. This driver is read-only, so
// each returns errReadOnly.

// errReadOnly returns the error for an unsupported write operation.
func errReadOnly(op string) error {
	return errz.Errorf("databricks: %s is not supported: the databricks driver is read-only "+
		"(native SQL via \"sq sql\" is not restricted)", op)
}

// PrepareInsertStmt implements driver.SQLDriver. It is not supported.
func (d *driveri) PrepareInsertStmt(_ context.Context, _ sqlz.DB, _ string, _ []string,
	_ int,
) (*driver.StmtExecer, error) {
	return nil, errReadOnly("insert")
}

// NewBatchInsert implements driver.SQLDriver. It is not supported.
func (d *driveri) NewBatchInsert(_ context.Context, _ string, _ sqlz.DB,
	_ *source.Source, _ string, _ []string,
) (*driver.BatchInsert, error) {
	return nil, errReadOnly("insert")
}

// PrepareUpdateStmt implements driver.SQLDriver. It is not supported.
func (d *driveri) PrepareUpdateStmt(_ context.Context, _ sqlz.DB, _ string, _ []string,
	_ string,
) (*driver.StmtExecer, error) {
	return nil, errReadOnly("update")
}

// CreateTable implements driver.SQLDriver. It is not supported.
func (d *driveri) CreateTable(_ context.Context, _ sqlz.DB, _ *schema.Table) error {
	return errReadOnly("create table")
}

// CreateSchema implements driver.SQLDriver. It is not supported.
func (d *driveri) CreateSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errReadOnly("create schema")
}

// DropSchema implements driver.SQLDriver. It is not supported.
func (d *driveri) DropSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errReadOnly("drop schema")
}

// Truncate implements driver.SQLDriver. It is not supported.
func (d *driveri) Truncate(_ context.Context, _ *source.Source, _ string, _ bool) (int64, error) {
	return 0, errReadOnly("truncate")
}

// CopyTable implements driver.SQLDriver. It is not supported.
func (d *driveri) CopyTable(_ context.Context, _ sqlz.DB, _, _ tablefq.T, _ bool) (int64, error) {
	return 0, errReadOnly("copy table")
}

// DropTable implements driver.SQLDriver. It is not supported.
func (d *driveri) DropTable(_ context.Context, _ sqlz.DB, _ tablefq.T, _ bool) error {
	return errReadOnly("drop table")
}

// AlterTableRename implements driver.SQLDriver. It is not supported.
func (d *driveri) AlterTableRename(_ context.Context, _ sqlz.DB, _, _ string) error {
	return errReadOnly("rename table")
}

// AlterTableAddColumn implements driver.SQLDriver. It is not supported.
func (d *driveri) AlterTableAddColumn(_ context.Context, _ sqlz.DB, _, _ string, _ kind.Kind) error {
	return errReadOnly("add column")
}

// AlterTableRenameColumn implements driver.SQLDriver. It is not supported.
func (d *driveri) AlterTableRenameColumn(_ context.Context, _ sqlz.DB, _, _, _ string) error {
	return errReadOnly("rename column")
}

// AlterTableColumnKinds implements driver.SQLDriver. It is not supported.
func (d *driveri) AlterTableColumnKinds(_ context.Context, _ sqlz.DB, _ string, _ []string, _ []kind.Kind) error {
	return errReadOnly("alter column kind")
}
