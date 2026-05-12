package duckdb

import (
	"context"
	"database/sql"
	"log/slog"

	_ "github.com/duckdb/duckdb-go/v2" // Import for side effect of loading the driver

	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/schema"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/driver/dialect"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (
	// dbDrvr is the backing duckdb SQL driver impl name.
	dbDrvr = "duckdb"

	// Prefix is the scheme+separator value "duckdb://".
	Prefix = "duckdb://"
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the DuckDB implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.DuckDB {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}
	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the DuckDB implementation of driver.SQLDriver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver.
// Whitelist of DuckDB settings that drive shell completion.
// See https://duckdb.org/docs/configuration/overview.
func (d *driveri) ConnParams() map[string][]string {
	return map[string][]string{
		"access_mode":            {"READ_ONLY", "READ_WRITE"},
		"memory_limit":           nil,
		"threads":                nil,
		"default_order":          {"ASC", "DESC"},
		"default_null_order":     {"NULLS_FIRST", "NULLS_LAST"},
		"enable_external_access": {"true", "false"},
		"enable_object_cache":    {"true", "false"},
		"temp_directory":         nil,
		"wal_autocheckpoint":     nil,
	}
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.DuckDB,
		Description: "DuckDB",
		Doc:         "https://duckdb.org",
		IsSQL:       true,
	}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)
	db, err := d.doOpen(ctx, src)
	if err != nil {
		return nil, errz.Err(err)
	}
	if err = driver.OpeningPing(ctx, src, db); err != nil {
		return nil, err
	}
	return &grip{log: d.log, db: db, src: src, drvr: d}, nil
}

func (d *driveri) doOpen(_ context.Context, src *source.Source) (*sql.DB, error) {
	dsn, err := dsnFromLocation(src.Location)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(dbDrvr, dsn)
	if err != nil {
		return nil, errz.Err(err)
	}
	return db, nil
}

// dsnFromLocation converts an sq location string ("duckdb:///path/to.duckdb?param=val")
// into the DSN form expected by go-duckdb.
// go-duckdb accepts either "" (in-memory) or a file path with optional "?key=val&..."
// query suffix. Strip the "duckdb://" prefix; preserve query string.
func dsnFromLocation(loc string) (string, error) {
	if loc == Prefix || loc == Prefix+":memory:" {
		return "", nil
	}
	if len(loc) < len(Prefix) {
		return "", errz.Errorf("invalid duckdb location: %q", loc)
	}
	return loc[len(Prefix):], nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	return errz.New("not implemented")
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	return nil, errz.New("not implemented")
}

// Dialect implements driver.SQLDriver.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return func(err error) error { return errz.Err(err) }
}

// Renderer implements driver.SQLDriver.
func (d *driveri) Renderer() *render.Renderer {
	return nil
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(_ context.Context, _ sqlz.DB) (string, error) {
	return "", errz.New("not implemented")
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New("not implemented")
}

// ListSchemaMetadata implements driver.SQLDriver.
func (d *driveri) ListSchemaMetadata(_ context.Context, _ sqlz.DB) ([]*metadata.Schema, error) {
	return nil, errz.New("not implemented")
}

// CurrentCatalog implements driver.SQLDriver.
func (d *driveri) CurrentCatalog(_ context.Context, _ sqlz.DB) (string, error) {
	return "", errz.New("not implemented")
}

// ListCatalogs implements driver.SQLDriver.
func (d *driveri) ListCatalogs(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New("not implemented")
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(_ context.Context, _ sqlz.DB, _ string, _ []string) ([]*sql.ColumnType, error) {
	return nil, errz.New("not implemented")
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(_ context.Context, _ []*sql.ColumnType) (record.Meta, driver.NewRecordFunc, error) {
	return nil, nil, errz.New("not implemented")
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(_ context.Context, _ sqlz.DB, _ string, _ []string, _ int) (*driver.StmtExecer, error) {
	return nil, errz.New("not implemented")
}

// NewBatchInsert implements driver.SQLDriver.
func (d *driveri) NewBatchInsert(_ context.Context, _ string, _ sqlz.DB, _ *source.Source, _ string, _ []string) (*driver.BatchInsert, error) {
	return nil, errz.New("not implemented")
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(_ context.Context, _ sqlz.DB, _ string, _ []string, _ string) (*driver.StmtExecer, error) {
	return nil, errz.New("not implemented")
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(_ context.Context, _ sqlz.DB, _ *schema.Table) error {
	return errz.New("not implemented")
}

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("not implemented")
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("not implemented")
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, errz.New("not implemented")
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, errz.New("not implemented")
}

// Truncate implements driver.SQLDriver.
func (d *driveri) Truncate(_ context.Context, _ *source.Source, _ string, _ bool) (int64, error) {
	return 0, errz.New("not implemented")
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, errz.New("not implemented")
}

// ListTableNames implements driver.SQLDriver.
func (d *driveri) ListTableNames(_ context.Context, _ sqlz.DB, _ string, _, _ bool) ([]string, error) {
	return nil, errz.New("not implemented")
}

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(_ context.Context, _ sqlz.DB, _, _ tablefq.T, _ bool) (int64, error) {
	return 0, errz.New("not implemented")
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(_ context.Context, _ sqlz.DB, _ tablefq.T, _ bool) error {
	return errz.New("not implemented")
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(_ context.Context, _ sqlz.DB, _, _ string) error {
	return errz.New("not implemented")
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(_ context.Context, _ sqlz.DB, _, _ string, _ kind.Kind) error {
	return errz.New("not implemented")
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(_ context.Context, _ sqlz.DB, _, _, _ string) error {
	return errz.New("not implemented")
}

// AlterTableColumnKinds implements driver.SQLDriver.
func (d *driveri) AlterTableColumnKinds(_ context.Context, _ sqlz.DB, _ string, _ []string, _ []kind.Kind) error {
	return errz.New("not implemented")
}

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(_ context.Context, _ sqlz.DB) (map[string]any, error) {
	return nil, errz.New("not implemented")
}

// grip is a minimal placeholder for the DuckDB Grip implementation.
// It will be replaced by a full implementation in Task 1.4.
type grip struct {
	log  *slog.Logger
	db   *sql.DB
	src  *source.Source
	drvr *driveri
}

var _ driver.Grip = (*grip)(nil)

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
func (g *grip) SourceMetadata(_ context.Context, _ bool) (*metadata.Source, error) {
	return nil, errz.New("not implemented")
}

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(_ context.Context, _ string) (*metadata.Table, error) {
	return nil, errz.New("not implemented")
}

// Close implements driver.Grip.
func (g *grip) Close() error {
	return errz.Err(g.db.Close())
}
