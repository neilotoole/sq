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

// pool implements driver.Pool. It implements a deferred ingest
// of the Excel data.
type pool struct {
	// REVISIT: do we need pool.log, or can we use lg.FromContext?
	log *slog.Logger

	src         *source.Source
	files       *source.Files
	backingPool driver.Pool
}

// DB implements driver.Pool.
func (p *pool) DB(ctx context.Context) (*sql.DB, error) {
	return p.backingPool.DB(ctx)
}

// SQLDriver implements driver.Pool.
func (p *pool) SQLDriver() driver.SQLDriver {
	return p.backingPool.SQLDriver()
}

// Source implements driver.Pool.
func (p *pool) Source() *source.Source {
	return p.src
}

// SourceMetadata implements driver.Pool.
func (p *pool) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	md, err := p.backingPool.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	md.Handle = p.src.Handle
	md.Driver = Type
	md.Location = p.src.Location
	if md.Name, err = source.LocationFileName(p.src); err != nil {
		return nil, err
	}
	md.FQName = md.Name

	if md.Size, err = p.files.Size(ctx, p.src); err != nil {
		return nil, err
	}

	return md, nil
}

// TableMetadata implements driver.Pool.
func (p *pool) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	return p.backingPool.TableMetadata(ctx, tblName)
}

// Close implements driver.Pool.
func (p *pool) Close() error {
	p.log.Debug(lgm.CloseDB, lga.Handle, p.src.Handle)

	return p.backingPool.Close()
}
