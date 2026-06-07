package parquet

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

var _ driver.Grip = (*grip)(nil)

// grip implements driver.Grip for a Parquet source. It wraps an in-memory
// DuckDB grip that holds a "data" view pointing at read_parquet(<path>).
// SourceMetadata is rewritten so the source reports as drivertype.Parquet
// rather than the underlying DuckDB.
type grip struct {
	log    *slog.Logger
	src    *source.Source
	files  *files.Files
	dbGrip driver.Grip
}

// DB implements driver.Grip.
func (g *grip) DB(ctx context.Context) (*sql.DB, error) {
	return g.dbGrip.DB(ctx)
}

// SQLDriver implements driver.Grip.
func (g *grip) SQLDriver() driver.SQLDriver {
	return g.dbGrip.SQLDriver()
}

// Source implements driver.Grip.
func (g *grip) Source() *source.Source {
	return g.src
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	md, err := g.dbGrip.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	md.Handle = g.src.Handle
	md.Driver = drivertype.Parquet
	md.Location = g.src.Location
	if md.Name, err = location.Filename(g.src.Location); err != nil {
		return nil, err
	}
	md.FQName = md.Name

	var size int64
	if size, err = g.files.Filesize(ctx, g.src); err != nil {
		return nil, err
	}
	md.Size = &size

	return md, nil
}

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	return g.dbGrip.TableMetadata(ctx, tblName)
}

// Close implements driver.Grip.
func (g *grip) Close() error {
	g.log.Debug(lgm.CloseDB, lga.Handle, g.src.Handle)
	return g.dbGrip.Close()
}
