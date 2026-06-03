// Package rqlite implements the sq driver for rqlite, the distributed
// SQLite-based database (see https://rqlite.io). The backing SQL driver
// is github.com/rqlite/gorqlite's stdlib (database/sql) adapter.
//
// Unlike sq's sqlite3 driver, rqlite is networked: there is no file
// mode. The source location is an HTTP URL using one of two schemes,
//
//	rqlite://user:pass@host:4001    (HTTP)
//	rqlites://user:pass@host:4001   (HTTPS)
//
// which are translated to gorqlite's expected http(s):// URLs at Open
// time.
//
// rqlite's HTTP API does not support interactive transactions, only
// atomic batches via /db/execute. gorqlite's database/sql driver
// surfaces Begin/Commit/Rollback as no-ops, which means sq code paths
// that rely on a real Tx (notably Truncate and NewBatchInsert) need
// rqlite-specific replacements rather than reusing the standard
// Tx-wrapped helpers.
package rqlite

import (
	"context"
	"database/sql"
	"log/slog"
	"net/url"
	"strings"

	_ "github.com/rqlite/gorqlite/stdlib" // Import for side effect of registering the "rqlite" sql driver.

	"github.com/neilotoole/sq/libsq/ast/render"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/jointype"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
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
	// dbDrvr is the name registered by gorqlite/stdlib via
	// sql.Register("rqlite", ...). It is intentionally identical to
	// drivertype.Rqlite's string value.
	dbDrvr = "rqlite"

	// Prefix is the scheme+separator for plain-HTTP rqlite sources.
	Prefix = "rqlite://"

	// PrefixSecure is the scheme+separator for HTTPS rqlite sources.
	PrefixSecure = "rqlites://"

	defaultPort = 4001

	errNotImplemented = "rqlite driver: not yet implemented"
)

var _ driver.Provider = (*Provider)(nil)

// Provider is the rqlite implementation of driver.Provider.
type Provider struct {
	Log *slog.Logger
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.Rqlite {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{log: p.Log}, nil
}

var _ driver.SQLDriver = (*driveri)(nil)

// driveri is the rqlite implementation of driver.SQLDriver.
type driveri struct {
	log *slog.Logger
}

// ConnParams implements driver.SQLDriver. The values come from rqlite's
// connection URL grammar (see gorqlite/conn.go) and the small set of
// rqlite features sq actively cares about.
func (d *driveri) ConnParams() map[string][]string {
	return map[string][]string{
		"level":                   {"none", "weak", "linearizable", "strong"},
		"disableClusterDiscovery": {"true", "false"},
	}
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *driveri) ErrWrapFunc() func(error) error {
	return errw
}

// DBProperties implements driver.SQLDriver. Full implementation lands
// alongside the metadata work; for now we return an empty map so callers
// like `sq inspect` don't fail.
func (d *driveri) DBProperties(_ context.Context, _ sqlz.DB) (map[string]any, error) {
	return map[string]any{}, nil
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.Rqlite,
		Description: "rqlite",
		Doc:         "https://rqlite.io",
		IsSQL:       true,
		DefaultPort: defaultPort,
	}
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
	dsn, err := dsnFromLocation(src.Location)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(dbDrvr, dsn)
	if err != nil {
		// Don't include dsn in the error: it may carry credentials.
		return nil, errz.Wrapf(errw(err), "failed to open rqlite source %s", src.Handle)
	}

	driver.ConfigureDB(ctx, db, src.Options)
	return db, nil
}

// Truncate implements driver.Driver.
//
// TODO(gh444): implement without sql.Tx — rqlite's HTTP API has no
// interactive transactions; emit DELETE FROM tbl via /db/execute and,
// if reset is true, also UPDATE sqlite_sequence to clear the counter.
func (d *driveri) Truncate(_ context.Context, _ *source.Source, _ string, _ bool) (int64, error) {
	return 0, errz.New(errNotImplemented + ": Truncate")
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != drivertype.Rqlite {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", drivertype.Rqlite, src.Type)
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

	if err = db.PingContext(ctx); err != nil {
		return errz.Wrapf(errw(err), "ping %s: %s", src.Handle, src.Location)
	}

	return nil
}

// Dialect implements driver.SQLDriver. rqlite speaks SQLite SQL, so the
// dialect mirrors the sqlite3 driver's.
func (d *driveri) Dialect() dialect.Dialect {
	return dialect.Dialect{
		Type:           drivertype.Rqlite,
		Placeholders:   placeholders,
		Enquote:        stringz.DoubleQuote,
		MaxBatchValues: 500,
		Ops:            dialect.DefaultOps(),
		ExecModeFor:    dialect.DefaultExecModeFor,
		Joins:          jointype.All(),
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

// Renderer implements driver.SQLDriver. The minimal version uses the
// default renderer; the SLQ function overrides (contains, like, etc.)
// land with the read-path work.
func (d *driveri) Renderer() *render.Renderer {
	return render.NewDefaultRenderer()
}

// dsnFromLocation translates an rqlite:// or rqlites:// source location
// into the http(s):// URL that gorqlite.Open expects.
func dsnFromLocation(loc string) (string, error) {
	var scheme string
	switch {
	case strings.HasPrefix(loc, PrefixSecure):
		scheme = "https"
	case strings.HasPrefix(loc, Prefix):
		scheme = "http"
	default:
		return "", errz.Errorf("rqlite: location must start with %q or %q, got: %s",
			Prefix, PrefixSecure, loc)
	}

	u, err := url.Parse(loc)
	if err != nil {
		return "", errz.Wrapf(err, "rqlite: invalid location: %s", loc)
	}

	u.Scheme = scheme
	return u.String(), nil
}

// ===== Stubs for SQLDriver methods. Each lands in a later task on the
// gh444-rqlite branch — see the task list on the branch for ordering.

// CopyTable implements driver.SQLDriver.
func (d *driveri) CopyTable(_ context.Context, _ sqlz.DB, _, _ tablefq.T, _ bool) (int64, error) {
	return 0, errz.New(errNotImplemented + ": CopyTable")
}

// RecordMeta implements driver.SQLDriver.
func (d *driveri) RecordMeta(_ context.Context, _ []*sql.ColumnType) (
	record.Meta, driver.NewRecordFunc, error,
) {
	return nil, nil, errz.New(errNotImplemented + ": RecordMeta")
}

// DropTable implements driver.SQLDriver.
func (d *driveri) DropTable(_ context.Context, _ sqlz.DB, _ tablefq.T, _ bool) error {
	return errz.New(errNotImplemented + ": DropTable")
}

// CreateSchema implements driver.SQLDriver.
func (d *driveri) CreateSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("rqlite: CREATE SCHEMA is not supported (SQLite has no schemas)")
}

// DropSchema implements driver.SQLDriver.
func (d *driveri) DropSchema(_ context.Context, _ sqlz.DB, _ string) error {
	return errz.New("rqlite: DROP SCHEMA is not supported (SQLite has no schemas)")
}

// CreateTable implements driver.SQLDriver.
func (d *driveri) CreateTable(_ context.Context, _ sqlz.DB, _ *schema.Table) error {
	return errz.New(errNotImplemented + ": CreateTable")
}

// CurrentSchema implements driver.SQLDriver.
func (d *driveri) CurrentSchema(_ context.Context, _ sqlz.DB) (string, error) {
	// Same convention as the sqlite3 driver: SQLite reports the
	// primary database as "main".
	return "main", nil
}

// SchemaExists implements driver.SQLDriver.
func (d *driveri) SchemaExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, errz.New(errNotImplemented + ": SchemaExists")
}

// ListSchemas implements driver.SQLDriver.
func (d *driveri) ListSchemas(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New(errNotImplemented + ": ListSchemas")
}

// ListTableNames implements driver.SQLDriver.
func (d *driveri) ListTableNames(_ context.Context, _ sqlz.DB, _ string, _, _ bool) ([]string, error) {
	return nil, errz.New(errNotImplemented + ": ListTableNames")
}

// ListSchemaMetadata implements driver.SQLDriver.
func (d *driveri) ListSchemaMetadata(_ context.Context, _ sqlz.DB) ([]*metadata.Schema, error) {
	return nil, errz.New(errNotImplemented + ": ListSchemaMetadata")
}

// CatalogExists implements driver.SQLDriver.
func (d *driveri) CatalogExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, nil
}

// CurrentCatalog implements driver.SQLDriver.
func (d *driveri) CurrentCatalog(_ context.Context, _ sqlz.DB) (string, error) {
	return "", errz.New("rqlite: catalogs are not supported (SQLite has no catalogs)")
}

// ListCatalogs implements driver.SQLDriver.
func (d *driveri) ListCatalogs(_ context.Context, _ sqlz.DB) ([]string, error) {
	return nil, errz.New("rqlite: catalogs are not supported (SQLite has no catalogs)")
}

// TableExists implements driver.SQLDriver.
func (d *driveri) TableExists(_ context.Context, _ sqlz.DB, _ string) (bool, error) {
	return false, errz.New(errNotImplemented + ": TableExists")
}

// PrepareInsertStmt implements driver.SQLDriver.
func (d *driveri) PrepareInsertStmt(_ context.Context, _ sqlz.DB, _ string, _ []string, _ int,
) (*driver.StmtExecer, error) {
	return nil, errz.New(errNotImplemented + ": PrepareInsertStmt")
}

// NewBatchInsert implements driver.SQLDriver.
func (d *driveri) NewBatchInsert(_ context.Context, _ string, _ sqlz.DB, _ *source.Source,
	_ string, _ []string,
) (*driver.BatchInsert, error) {
	return nil, errz.New(errNotImplemented + ": NewBatchInsert")
}

// PrepareUpdateStmt implements driver.SQLDriver.
func (d *driveri) PrepareUpdateStmt(_ context.Context, _ sqlz.DB, _ string, _ []string, _ string,
) (*driver.StmtExecer, error) {
	return nil, errz.New(errNotImplemented + ": PrepareUpdateStmt")
}

// TableColumnTypes implements driver.SQLDriver.
func (d *driveri) TableColumnTypes(_ context.Context, _ sqlz.DB, _ string, _ []string) ([]*sql.ColumnType, error) {
	return nil, errz.New(errNotImplemented + ": TableColumnTypes")
}

// AlterTableRename implements driver.SQLDriver.
func (d *driveri) AlterTableRename(_ context.Context, _ sqlz.DB, _, _ string) error {
	return errz.New(errNotImplemented + ": AlterTableRename")
}

// AlterTableAddColumn implements driver.SQLDriver.
func (d *driveri) AlterTableAddColumn(_ context.Context, _ sqlz.DB, _, _ string, _ kind.Kind) error {
	return errz.New(errNotImplemented + ": AlterTableAddColumn")
}

// AlterTableRenameColumn implements driver.SQLDriver.
func (d *driveri) AlterTableRenameColumn(_ context.Context, _ sqlz.DB, _, _, _ string) error {
	return errz.New(errNotImplemented + ": AlterTableRenameColumn")
}

// AlterTableColumnKinds implements driver.SQLDriver.
func (d *driveri) AlterTableColumnKinds(_ context.Context, _ sqlz.DB, _ string, _ []string, _ []kind.Kind) error {
	return errz.New(errNotImplemented + ": AlterTableColumnKinds")
}
