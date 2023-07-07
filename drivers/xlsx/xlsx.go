// Package xlsx implements the sq driver for Microsoft Excel.
package xlsx

import (
	"context"
	"database/sql"
	"io"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/tealeg/xlsx/v2"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// Type is the sq source driver type for XLSX.
	Type = source.DriverType("xlsx")
)

// Provider implements driver.Provider.
type Provider struct {
	Log       *slog.Logger
	Files     *source.Files
	Scratcher driver.ScratchDatabaseOpener
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
	scratcher driver.ScratchDatabaseOpener
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

// Open implements driver.DatabaseOpener.
func (d *Driver) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	r, err := d.files.Open(src)
	if err != nil {
		return nil, err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	b, err := io.ReadAll(r)
	if err != nil {
		return nil, errz.Err(err)
	}

	xlFile, err := xlsx.OpenBinary(b)
	if err != nil {
		return nil, err
	}

	scratchDB, err := d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		return nil, err
	}

	clnup := cleanup.New()
	clnup.AddE(scratchDB.Close)

	// REVISIT: Can we defer ingest?
	err = ingest(ctx, src, scratchDB, xlFile, nil)
	if err != nil {
		lg.WarnIfError(d.log, lgm.CloseDB, clnup.Run())
		return nil, err
	}

	return &database{log: d.log, src: src, impl: scratchDB, files: d.files, clnup: clnup}, nil
}

// Truncate implements driver.Driver.
func (d *Driver) Truncate(_ context.Context, src *source.Source, _ string, _ bool) (affected int64, err error) {
	// TODO: Ww could actually implement Truncate for xlsx.
	// It would just mean deleting the rows from a sheet, and then
	// saving the sheet. But that's probably not a game we want to
	// get into, as sq doesn't currently make writes to any non-SQL
	// source types.
	return 0, errz.Errorf("driver type {%s} (%s) doesn't support dropping tables", Type, src.Handle)
}

// ValidateSource implements driver.Driver.
func (d *Driver) ValidateSource(src *source.Source) (*source.Source, error) {
	d.log.Debug("Validating source: {%s}", src.RedactedLocation())
	if src.Type != Type {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", Type, src.Type)
	}

	return src, nil
}

// Ping implements driver.Driver.
func (d *Driver) Ping(_ context.Context, src *source.Source) (err error) {
	r, err := d.files.Open(src)
	if err != nil {
		return err
	}

	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	b, err := io.ReadAll(r)
	if err != nil {
		return errz.Err(err)
	}

	_, err = xlsx.OpenBinaryWithRowLimit(b, 1)
	if err != nil {
		return errz.Err(err)
	}

	return nil
}

// database implements driver.Database.
type database struct {
	log   *slog.Logger
	src   *source.Source
	files *source.Files
	impl  driver.Database
	clnup *cleanup.Cleanup
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

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context, noSchema bool) (*source.Metadata, error) {
	md, err := d.impl.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	md.Handle = d.src.Handle
	md.Driver = Type
	md.Location = d.src.Location
	if md.Name, err = source.LocationFileName(d.src); err != nil {
		return nil, err
	}
	md.FQName = md.Name

	if md.Size, err = d.files.Size(d.src); err != nil {
		return nil, err
	}

	return md, nil
}

// TableMetadata implements driver.Database.
func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	srcMeta, err := d.SourceMetadata(ctx, false)
	if err != nil {
		return nil, err
	}

	tblMeta := srcMeta.Table(tblName)
	if tblMeta == nil {
		return nil, errz.Errorf("table {%s} not found", tblName)
	}

	return tblMeta, nil
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debug(lgm.CloseDB, lga.Handle, d.src.Handle)

	// No need to explicitly invoke c.impl.Close because
	// that's already added to c.clnup
	return d.clnup.Run()
}
