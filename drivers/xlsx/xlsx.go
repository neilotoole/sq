// Package xlsx implements the sq driver for Microsoft Excel.
package xlsx

import (
	"context"
	"log/slog"

	excelize "github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// Type is the sq source driver type for XLSX.
	Type = source.DriverType("xlsx")

	// laSheet is a constant for the "sheet" log attribute.
	laSheet = "sheet"
)

// Provider implements driver.Provider.
type Provider struct {
	Log       *slog.Logger
	Files     *source.Files
	Scratcher driver.ScratchPoolOpener
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ source.DriverType) (driver.Driver, error) {
	if typ != Type {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &Driver{log: p.Log, scratcher: p.Scratcher, files: p.Files}, nil
}

// Driver implements driver.Driver.
type Driver struct {
	log       *slog.Logger
	scratcher driver.ScratchPoolOpener
	files     *source.Files
}

// DriverMetadata implements driver.Driver.
func (d *Driver) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        Type,
		Description: "Microsoft Excel XLSX",
		Doc:         "https://en.wikipedia.org/wiki/Microsoft_Excel",
	}
}

// Open implements driver.PoolOpener.
func (d *Driver) Open(ctx context.Context, src *source.Source) (driver.Pool, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	scratchPool, err := d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		return nil, err
	}

	clnup := cleanup.New()
	clnup.AddE(scratchPool.Close)

	p := &pool{
		log:         d.log,
		src:         src,
		scratchPool: scratchPool,
		files:       d.files,
		clnup:       clnup,
	}

	return p, nil
}

// Truncate implements driver.Driver.
func (d *Driver) Truncate(_ context.Context, src *source.Source, _ string, _ bool) (affected int64, err error) {
	// NOTE: We could actually implement Truncate for xlsx.
	// It would just mean deleting the rows from a sheet, and then
	// saving the sheet. But that's probably not a game we want to
	// get into, as sq doesn't currently make edits to any non-SQL
	// source types.
	return 0, errz.Errorf("driver type {%s} (%s) doesn't support dropping tables", Type, src.Handle)
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

	r, err := d.files.Open(src)
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
