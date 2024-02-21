package mysql

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// grip implements driver.Grip.
type grip struct {
	log       *slog.Logger
	db        *sql.DB
	src       *source.Source
	drvr      *driveri
	closeErr  error
	closeOnce sync.Once
}

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

	return getTableMetadata(ctx, g.db, tblName)
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	return getSourceMetadata(ctx, g.src, g.db, noSchema)
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
