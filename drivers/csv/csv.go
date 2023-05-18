// Package csv implements the sq driver for CSV/TSV et al.
package csv

import (
	"context"
	"database/sql"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// TypeCSV is the CSV driver type.
	TypeCSV = source.DriverType("csv")

	// TypeTSV is the TSV driver type.
	TypeTSV = source.DriverType("tsv")
)

// Provider implements driver.Provider.
type Provider struct {
	Log       *slog.Logger
	Scratcher driver.ScratchDatabaseOpener
	Files     *source.Files
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ source.DriverType) (driver.Driver, error) {
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
	typ       source.DriverType
	scratcher driver.ScratchDatabaseOpener
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

// Open implements driver.DatabaseOpener.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	dbase := &database{
		log:   d.log,
		src:   src,
		files: d.files,
	}

	var err error
	dbase.impl, err = d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		return nil, err
	}

	if err = ingestCSV(ctx, src, d.files.OpenFunc(src), dbase.impl); err != nil {
		return nil, err
	}

	return dbase, nil
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
func (d *driveri) Ping(_ context.Context, src *source.Source) error {
	r, err := d.files.Open(src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	return nil
}

// database implements driver.Database.
type database struct {
	log   *slog.Logger
	src   *source.Source
	impl  driver.Database
	files *source.Files
}

// DB implements driver.Database.
func (d *database) DB() *sql.DB {
	return d.impl.DB()
}

// SQLDriver implements driver.Database.
func (d *database) SQLDriver() driver.SQLDriver {
	return d.impl.SQLDriver()
}

// Source implements driver.Database.
func (d *database) Source() *source.Source {
	return d.src
}

// TableMetadata implements driver.Database.
func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	if tblName != source.MonotableName {
		return nil, errz.Errorf("table name should be %s for CSV/TSV etc., but got: %s",
			source.MonotableName, tblName)
	}

	srcMeta, err := d.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}

	// There will only ever be one table for CSV.
	return srcMeta.Tables[0], nil
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	md, err := d.impl.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}

	md.Handle = d.src.Handle
	md.Location = d.src.Location
	md.Driver = d.src.Type

	md.Name, err = source.LocationFileName(d.src)
	if err != nil {
		return nil, err
	}

	md.Size, err = d.files.Size(d.src)
	if err != nil {
		return nil, err
	}

	md.FQName = md.Name
	return md, nil
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debug(lgm.CloseDB, lga.Handle, d.src.Handle)

	return errz.Err(d.impl.Close())
}
