// Package xlsx implements the sq driver for Microsoft Excel.
package xlsx

import (
	"context"
	"log/slog"

	"github.com/neilotoole/sq/libsq/files"

	excelize "github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

const (

	// laSheet is a constant for the "sheet" log attribute.
	laSheet = "sheet"
)

// Provider implements driver.Provider.
type Provider struct {
	Log      *slog.Logger
	Files    *files.Files
	Ingester driver.GripOpenIngester
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.XLSX {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &Driver{log: p.Log, ingester: p.Ingester, files: p.Files}, nil
}

// Driver implements driver.Driver.
type Driver struct {
	log      *slog.Logger
	ingester driver.GripOpenIngester
	files    *files.Files
}

// DriverMetadata implements driver.Driver.
func (d *Driver) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.XLSX,
		Description: "Microsoft Excel XLSX",
		Doc:         "https://en.wikipedia.org/wiki/Microsoft_Excel",
	}
}

// Open implements driver.Driver.
func (d *Driver) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	log := lg.FromContext(ctx).With(lga.Src, src)
	log.Debug(lgm.OpenSrc, lga.Src, src)

	p := &grip{
		log:   log,
		src:   src,
		files: d.files,
	}

	allowCache := driver.OptIngestCache.Get(options.FromContext(ctx))

	ingestFn := func(ctx context.Context, destGrip driver.Grip) error {
		log.Debug("Ingest XLSX", lga.Src, p.src)
		r, err := p.files.NewReader(ctx, p.src, false)
		if err != nil {
			return err
		}
		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

		xfile, err := excelize.OpenReader(r, excelize.Options{RawCellValue: false})
		if err != nil {
			return err
		}

		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, xfile)

		return ingestXLSX(ctx, p.src, destGrip, xfile)
	}

	var err error
	if p.dbGrip, err = d.ingester.OpenIngest(ctx, p.src, allowCache, ingestFn); err != nil {
		return nil, err
	}

	return p, nil
}

// ValidateSource implements driver.Driver.
func (d *Driver) ValidateSource(src *source.Source) (*source.Source, error) {
	d.log.Debug("Validating source", lga.Src, src)
	if src.Type != drivertype.XLSX {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", drivertype.XLSX, src.Type)
	}

	return src, nil
}

// Ping implements driver.Driver.
func (d *Driver) Ping(ctx context.Context, src *source.Source) (err error) {
	return d.files.Ping(ctx, src)
}

func errw(err error) error {
	return errz.Wrap(err, "excel")
}
