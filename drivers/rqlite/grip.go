package rqlite

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// grip implements driver.Grip.
type grip struct {
	closeErr  error
	log       *slog.Logger
	db        *sql.DB
	src       *source.Source
	drvr      *driveri
	drvrw     *enrichingSQLDriver
	semver    driver.SemverCache
	closeOnce sync.Once
}

// DB implements driver.Grip.
func (g *grip) DB(context.Context) (*sql.DB, error) {
	return g.db, nil
}

// SQLDriver implements driver.Grip. It returns the package driveri
// wrapped so that ErrWrapFunc can enrich errors with source-specific
// hints: libsq obtains its error-wrap func via
// grip.SQLDriver().ErrWrapFunc() on the query path, and the
// connection-error enrichments (cluster discovery, TLS, cert) need
// the source's host and query params to apply their guards.
func (g *grip) SQLDriver() driver.SQLDriver {
	return g.drvrw
}

// enrichingSQLDriver wraps the source-agnostic driveri so that
// ErrWrapFunc closes over the grip's source. It embeds the concrete
// *driveri (not the SQLDriver interface) so that optional interfaces
// implemented by driveri, e.g. driver.ConnParamDetector, remain
// visible through type assertions on the wrapper. The grip constructs
// one wrapper at Open and reuses it.
type enrichingSQLDriver struct {
	*driveri
	src *source.Source
}

// ErrWrapFunc implements driver.SQLDriver.
func (d *enrichingSQLDriver) ErrWrapFunc() func(error) error {
	return func(err error) error {
		return enrichConnError(errw(err), d.src)
	}
}

// Source implements driver.Grip.
func (g *grip) Source() *source.Source {
	return g.src
}

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	md, err := getTableMetadata(ctx, g.db, tblName)
	return md, enrichConnError(err, g.src)
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	md, err := getSourceMetadata(ctx, g.src, g.db, noSchema)
	return md, enrichConnError(err, g.src)
}

// DBSemver implements driver.Grip.
func (g *grip) DBSemver(ctx context.Context) (string, error) {
	return g.semver.Get(func() (string, error) { return g.drvr.DBSemver(ctx, g.db) })
}

// Close implements driver.Grip. Subsequent calls to Close are no-op and
// return the same error.
func (g *grip) Close() error {
	g.closeOnce.Do(func() {
		g.closeErr = errw(g.db.Close())
		if g.closeErr != nil {
			g.log.Error(lgm.CloseDB, lga.Handle, g.src.Handle, lga.Err, g.closeErr)
		} else {
			g.log.Debug(lgm.CloseDB, lga.Handle, g.src.Handle)
		}
	})

	return g.closeErr
}
