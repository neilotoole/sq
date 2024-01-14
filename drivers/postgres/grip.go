package postgres

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// grip is the postgres implementation of driver.Grip.
type grip struct {
	log  *slog.Logger
	drvr *driveri
	db   *sql.DB
	src  *source.Source
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
	db, err := g.DB(ctx)
	if err != nil {
		return nil, err
	}

	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+"."+tblName+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	return getTableMetadata(ctx, db, tblName)
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	db, err := g.DB(ctx)
	if err != nil {
		return nil, err
	}

	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	return getSourceMetadata(ctx, g.src, db, noSchema)
}

// Close implements driver.Grip.
func (g *grip) Close() error {
	g.log.Debug(lgm.CloseDB, lga.Handle, g.src.Handle)

	err := g.db.Close()
	if err != nil {
		return errw(err)
	}
	return nil
}
