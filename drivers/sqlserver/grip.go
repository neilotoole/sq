package sqlserver

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/neilotoole/sq/libsq/core/debugz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
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
	semver    driver.SemverCache
	closeOnce sync.Once
}

var _ driver.Grip = (*grip)(nil)

// DB implements driver.Grip.
func (g *grip) DB(context.Context) (*sql.DB, error) {
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

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+"."+tblName+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	const query = `SELECT TABLE_CATALOG, TABLE_SCHEMA, TABLE_TYPE
FROM INFORMATION_SCHEMA.TABLES
WHERE TABLE_NAME = @p1`

	var catalog, schema, tblType string
	err := g.db.QueryRowContext(ctx, query, tblName).Scan(&catalog, &schema, &tblType)
	if err != nil {
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)
	debugz.DebugSleep(ctx)

	// The per-table catalog queries can lose a lock race against concurrent
	// DDL and be chosen as a deadlock victim (error 1205); retry.
	return loadWithRetry(ctx, func() (*metadata.Table, error) {
		return getTableMetadata(ctx, g.db, catalog, schema, tblName, tblType, true)
	})
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	return getSourceMetadata(ctx, g.src, g.db, noSchema)
}

// DBSemver implements driver.Grip.
func (g *grip) DBSemver(ctx context.Context) (string, error) {
	return g.semver.Get(func() (string, error) { return g.drvr.DBSemver(ctx, g.db) })
}

// Close implements driver.Grip.
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
