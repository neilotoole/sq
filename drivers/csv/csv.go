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
	Log      *slog.Logger
	Ingester driver.GripOpenIngester
	Files    *source.Files
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	switch typ { //nolint:exhaustive
	case TypeCSV:
		return &driveri{log: d.Log, typ: TypeCSV, ingester: d.Ingester, files: d.Files}, nil
	case TypeTSV:
		return &driveri{log: d.Log, typ: TypeTSV, ingester: d.Ingester, files: d.Files}, nil
	}

	return nil, errz.Errorf("unsupported driver type {%s}", typ)
}

// Driver implements driver.Driver.
type driveri struct {
	log      *slog.Logger
	typ      drivertype.Type
	ingester driver.GripOpenIngester
	files    *source.Files
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

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	log := lg.FromContext(ctx)
	log.Debug(lgm.OpenSrc, lga.Src, src)

	g := &grip{
		log:   d.log,
		src:   src,
		files: d.files,
	}

	allowCache := driver.OptIngestCache.Get(options.FromContext(ctx))

	ingestFn := func(ctx context.Context, destGrip driver.Grip) error {
		openFn := d.files.OpenFunc(src)
		log.Debug("Ingest func invoked", lga.Src, src)
		return ingestCSV(ctx, src, openFn, destGrip)
	}

	var err error
	if g.impl, err = d.ingester.OpenIngest(ctx, src, allowCache, ingestFn); err != nil {
		return nil, err
	}

	return g, nil
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
	return d.files.Ping(ctx, src)
}

// grip implements driver.Grip.
type grip struct {
	log   *slog.Logger
	src   *source.Source
	impl  driver.Grip
	files *source.Files
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
	if tblName != source.MonotableName {
		return nil, errz.Errorf("table name should be %s for CSV/TSV etc., but got: %s",
			source.MonotableName, tblName)
	}

	srcMeta, err := g.SourceMetadata(ctx, false)
	if err != nil {
		return nil, err
	}

	// There will only ever be one table for CSV.
	return srcMeta.Tables[0], nil
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	md, err := g.impl.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	md.Handle = g.src.Handle
	md.Location = g.src.Location
	md.Driver = g.src.Type

	md.Name, err = source.LocationFileName(g.src)
	if err != nil {
		return nil, err
	}

	md.Size, err = g.files.Filesize(ctx, g.src)
	if err != nil {
		return nil, err
	}

	md.FQName = md.Name
	return md, nil
}

// Close implements driver.Grip.
func (g *grip) Close() error {
	g.log.Debug(lgm.CloseDB, lga.Handle, g.src.Handle)

	return errz.Err(g.impl.Close())
}
