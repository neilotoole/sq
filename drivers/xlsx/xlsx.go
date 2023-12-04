// Package xlsx implements the sq driver for Microsoft Excel.
package xlsx

import (
	"context"
	"log/slog"

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
	// Type is the sq source driver type for XLSX.
	Type = drivertype.Type("xlsx")

	// laSheet is a constant for the "sheet" log attribute.
	laSheet = "sheet"
)

// Provider implements driver.Provider.
type Provider struct {
	Log      *slog.Logger
	Files    *source.Files
	Ingester driver.GripOpenIngester
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &Driver{log: p.Log, ingester: p.Ingester, files: p.Files}, nil
}

// Driver implements driver.Driver.
type Driver struct {
	log      *slog.Logger
	ingester driver.GripOpenIngester
	files    *source.Files
}

// DriverMetadata implements driver.Driver.
func (d *Driver) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        Type,
		Description: "Microsoft Excel XLSX",
		Doc:         "https://en.wikipedia.org/wiki/Microsoft_Excel",
	}
}

// Open implements driver.GripOpener.
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
		r, err := p.files.Open(ctx, p.src)
		if err != nil {
			return err
		}
		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

		xfile, err := excelize.OpenReader(r, excelize.Options{RawCellValue: false})
		if err != nil {
			return err
		}

		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, xfile)

		if err = ingestXLSX(ctx, p.src, destGrip, xfile, nil); err != nil {
			return err
		}
		return nil
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
	if src.Type != Type {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", Type, src.Type)
	}

	return src, nil
}

// Ping implements driver.Driver.
func (d *Driver) Ping(ctx context.Context, src *source.Source) (err error) {
	log := lg.FromContext(ctx)

	r, err := d.files.Open(ctx, src)
	if err != nil {
		return err
	}

	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	f, err := excelize.OpenReader(r)
	if err != nil {
		return errz.Err(err)
	}

	lg.WarnIfCloseError(log, lgm.CloseFileReader, f)

	return nil
}

func errw(err error) error {
	return errz.Wrap(err, "excel")
}
