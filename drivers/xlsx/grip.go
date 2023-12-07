package xlsx

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// grip implements driver.Grip. It implements a deferred ingest
// of the Excel data.
type grip struct {
	// REVISIT: do we need grip.log, or can we use lg.FromContext?
	log *slog.Logger

	src    *source.Source
	files  *source.Files
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
	md.Driver = Type
	md.Location = g.src.Location
	if md.Name, err = source.LocationFileName(g.src); err != nil {
		return nil, err
	}
	md.FQName = md.Name

	if md.Size, err = g.files.Filesize(ctx, g.src); err != nil {
		return nil, err
	}

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
