package clickhouse

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

var _ driver.Grip = (*grip)(nil)

// grip implements driver.Grip, which is a handle to an open database connection.
// It provides access to the underlying sql.DB, the source configuration, and
// metadata retrieval functions.
//
// The grip is returned by driveri.Open and should be closed when no longer
// needed to release the database connection.
type grip struct {
	// log is the logger for grip operations.
	log *slog.Logger

	// db is the underlying database connection.
	db *sql.DB

	// src is the source configuration for this connection.
	src *source.Source

	// drvr is the driver that created this grip, used for accessing
	// driver-level functionality.
	drvr *driveri

	// closeErr stores the error from closing the database connection.
	closeErr error

	// closeOnce ensures the database connection is closed only once.
	closeOnce sync.Once
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

// SourceMetadata implements driver.Grip. It retrieves metadata for the source,
// including database version, current database, and optionally table/column
// information.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	return getSourceMetadata(ctx, g.src, g.db, noSchema)
}

// TableMetadata implements driver.Grip. It retrieves metadata for a specific
// table, including column information.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+"."+tblName+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	return getTableMetadata(ctx, g.db, "", tblName)
}

// Close implements driver.Grip. It closes the database connection, ensuring
// the close only happens once even if called multiple times.
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
