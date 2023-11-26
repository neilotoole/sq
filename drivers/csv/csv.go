// Package csv implements the sq driver for CSV/TSV et al.
package csv

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (
	// TypeCSV is the CSV driver type.
	TypeCSV = drivertype.Type("csv")

	// TypeTSV is the TSV driver type.
	TypeTSV = drivertype.Type("tsv")
)

// Provider implements driver.Provider.
type Provider struct {
	Log       *slog.Logger
	Scratcher driver.ScratchPoolOpener
	Files     *source.Files
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	switch typ { //nolint:exhaustive
	case TypeCSV:
		return &driveri{log: d.Log, typ: TypeCSV, scratcher: d.Scratcher, files: d.Files}, nil
	case TypeTSV:
		return &driveri{log: d.Log, typ: TypeTSV, scratcher: d.Scratcher, files: d.Files}, nil
	}

	return nil, errz.Errorf("unsupported driver type {%s}", typ)
}

// Driver implements driver.Driver.
type driveri struct {
	log       *slog.Logger
	typ       drivertype.Type
	scratcher driver.ScratchPoolOpener
	files     *source.Files
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	md := driver.Metadata{Type: d.typ, Monotable: true}
	if d.typ == TypeCSV {
		md.Description = "Comma-Separated Values"
		md.Doc = "https://en.wikipedia.org/wiki/Comma-separated_values"
	} else {
		md.Description = "Tab-Separated Values"
		md.Doc = "https://en.wikipedia.org/wiki/Tab-separated_values"
	}
	return md
}

// Open implements driver.PoolOpener.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Pool, error) {
	log := lg.FromContext(ctx)
	log.Debug(lgm.OpenSrc, lga.Src, src)

	p := &pool{
		log:   d.log,
		src:   src,
		files: d.files,
	}

	allowCache := driver.OptIngestCache.Get(options.FromContext(ctx))

	ingestFn := func(ctx context.Context, destPool driver.Pool) error {
		return ingestCSV(ctx, src, d.files.OpenFunc(src), destPool)
	}

	backingPool, err := d.scratcher.OpenIngest(ctx, src, ingestFn, allowCache)
	if err != nil {
		return nil, err
	}

	p.impl = backingPool
	return p, nil
}

// Truncate implements driver.Driver.
func (d *driveri) Truncate(_ context.Context, _ *source.Source, _ string, _ bool) (int64, error) {
	return 0, errz.Errorf("truncate not supported for %s", d.DriverMetadata().Type)
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != d.typ {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", d.typ, src.Type)
	}

	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	// FIXME: Does Ping calling d.files.Open cause a full read?
	// We probably just want to check that the file exists
	// or is accessible.
	r, err := d.files.Open(ctx, src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	return nil
}

// pool implements driver.Pool.
type pool struct {
	log   *slog.Logger
	src   *source.Source
	impl  driver.Pool
	files *source.Files
}

// DB implements driver.Pool.
func (p *pool) DB(ctx context.Context) (*sql.DB, error) {
	return p.impl.DB(ctx)
}

// SQLDriver implements driver.Pool.
func (p *pool) SQLDriver() driver.SQLDriver {
	return p.impl.SQLDriver()
}

// Source implements driver.Pool.
func (p *pool) Source() *source.Source {
	return p.src
}

// TableMetadata implements driver.Pool.
func (p *pool) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	if tblName != source.MonotableName {
		return nil, errz.Errorf("table name should be %s for CSV/TSV etc., but got: %s",
			source.MonotableName, tblName)
	}

	srcMeta, err := p.SourceMetadata(ctx, false)
	if err != nil {
		return nil, err
	}

	// There will only ever be one table for CSV.
	return srcMeta.Tables[0], nil
}

// SourceMetadata implements driver.Pool.
func (p *pool) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	md, err := p.impl.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	md.Handle = p.src.Handle
	md.Location = p.src.Location
	md.Driver = p.src.Type

	md.Name, err = source.LocationFileName(p.src)
	if err != nil {
		return nil, err
	}

	md.Size, err = p.files.Size(ctx, p.src)
	if err != nil {
		return nil, err
	}

	md.FQName = md.Name
	return md, nil
}

// Close implements driver.Pool.
func (p *pool) Close() error {
	p.log.Debug(lgm.CloseDB, lga.Handle, p.src.Handle)

	return errz.Err(p.impl.Close())
}
