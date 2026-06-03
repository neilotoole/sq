package rqlite

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// grip implements driver.Grip.
type grip struct {
	closeErr  error
	log       *slog.Logger
	db        *sql.DB
	src       *source.Source
	drvr      *driveri
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
func (g *grip) TableMetadata(_ context.Context, _ string) (*metadata.Table, error) {
	return nil, errz.New("rqlite: TableMetadata not yet implemented")
}

// SourceMetadata implements driver.Grip. The current implementation
// returns a skeletal record with just enough fields populated for
// `sq ping` and basic source listing. Full metadata (tables, columns,
// FKs) lands when the read-path work in task gh444-#5 completes.
func (g *grip) SourceMetadata(_ context.Context, _ bool) (*metadata.Source, error) {
	return &metadata.Source{
		Handle:    g.src.Handle,
		Driver:    drivertype.Rqlite,
		DBDriver:  dbDrvr,
		Location:  g.src.Location,
		DBProduct: "rqlite",
		Catalog:   "default",
	}, nil
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
