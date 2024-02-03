package userdriver

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/location"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// grip implements driver.Grip.
type grip struct {
	log  *slog.Logger
	src  *source.Source
	impl driver.Grip
}

// DB implements driver.Grip.
func (g *grip) DB(ctx context.Context) (*sql.DB, error) {
	return g.impl.DB(ctx)
}

// SQLDriver implements driver.Grip.
func (g *grip) SQLDriver() driver.SQLDriver {
	return g.impl.SQLDriver()
}

// Source implements driver.Grip.
func (g *grip) Source() *source.Source {
	return g.src
}

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	return g.impl.TableMetadata(ctx, tblName)
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	md, err := g.impl.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	md.Handle = g.src.Handle
	md.Location = g.src.Location
	md.Name, err = location.Filename(g.src.Location)
	if err != nil {
		return nil, err
	}

	md.FQName = md.Name
	return md, nil
}

// Close implements driver.Grip.
func (g *grip) Close() error {
	g.log.Debug(lgm.CloseDB, lga.Handle, g.src.Handle)

	return g.impl.Close()
}
