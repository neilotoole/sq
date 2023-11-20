// Package userdriver implements the "user-driver" functionality
// that allows users to define source driver types declaratively.
// Note pkg userdriver itself is the framework: an actual
// implementation for each genre (such as XML) must be defined
// separately as in the "xmlud" sub-package.
package userdriver

import (
	"context"
	"database/sql"
	"io"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// ImportFunc is a function that can import
// data (as defined in def) to destPool.
type ImportFunc func(ctx context.Context, def *DriverDef,
	data io.Reader, destPool driver.Pool) error

// Provider implements driver.Provider for a DriverDef.
type Provider struct {
	Log       *slog.Logger
	DriverDef *DriverDef
	Scratcher driver.ScratchPoolOpener
	Files     *source.Files
	ImportFn  ImportFunc
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ source.DriverType) (driver.Driver, error) {
	if typ != source.DriverType(p.DriverDef.Name) {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{
		log:       p.Log,
		typ:       typ,
		def:       p.DriverDef,
		scratcher: p.Scratcher,
		importFn:  p.ImportFn,
		files:     p.Files,
	}, nil
}

// Detectors returns funcs that can detect the driver type.
func (p *Provider) Detectors() []source.DriverDetectFunc {
	// TODO: it should be possible to return type detectors that
	//  can detect based upon the DriverDef. So, as of right
	//  now these detectors do nothing.
	return []source.DriverDetectFunc{}
}

// Driver implements driver.Driver.
type driveri struct {
	log       *slog.Logger
	typ       source.DriverType
	def       *DriverDef
	files     *source.Files
	scratcher driver.ScratchPoolOpener
	importFn  ImportFunc
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        source.DriverType(d.def.Name),
		Description: d.def.Title,
		Doc:         d.def.Doc,
		UserDefined: true,
	}
}

// Open implements driver.PoolOpener.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Pool, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	clnup := cleanup.New()

	r, err := d.files.Open(src)
	if err != nil {
		return nil, err
	}

	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	scratchDB, err := d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		return nil, err
	}
	clnup.AddE(scratchDB.Close)

	err = d.importFn(ctx, d.def, r, scratchDB)
	if err != nil {
		lg.WarnIfFuncError(d.log, lgm.CloseDB, clnup.Run)
		return nil, errz.Wrap(err, d.def.Name)
	}

	return &pool{log: d.log, src: src, impl: scratchDB, clnup: clnup}, nil
}

// Truncate implements driver.Driver.
func (d *driveri) Truncate(_ context.Context, _ *source.Source, _ string, _ bool) (int64, error) {
	return 0, errz.Errorf("truncate not supported for %s", d.DriverMetadata().Type)
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	d.log.Debug("Validating source", lga.Src, src)
	if string(src.Type) != d.def.Name {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}", d.def.Name, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(_ context.Context, src *source.Source) error {
	d.log.Debug("Ping source",
		lga.Driver, d.typ,
		lga.Src, src,
	)

	r, err := d.files.Open(src)
	if err != nil {
		return err
	}

	// TODO: possibly do something more useful than just
	//  getting the reader?

	return r.Close()
}

// pool implements driver.Pool.
type pool struct {
	log  *slog.Logger
	src  *source.Source
	impl driver.Pool

	// clnup will ultimately invoke impl.Close to dispose of
	// the scratch DB.
	clnup *cleanup.Cleanup
}

// DB implements driver.Pool.
func (d *pool) DB(ctx context.Context) (*sql.DB, error) {
	return d.impl.DB(ctx)
}

// SQLDriver implements driver.Pool.
func (d *pool) SQLDriver() driver.SQLDriver {
	return d.impl.SQLDriver()
}

// Source implements driver.Pool.
func (d *pool) Source() *source.Source {
	return d.src
}

// TableMetadata implements driver.Pool.
func (d *pool) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	return d.impl.TableMetadata(ctx, tblName)
}

// SourceMetadata implements driver.Pool.
func (d *pool) SourceMetadata(ctx context.Context, noSchema bool) (*source.Metadata, error) {
	meta, err := d.impl.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	meta.Handle = d.src.Handle
	meta.Location = d.src.Location
	meta.Name, err = source.LocationFileName(d.src)
	if err != nil {
		return nil, err
	}

	meta.FQName = meta.Name
	return meta, nil
}

// Close implements driver.Pool.
func (d *pool) Close() error {
	d.log.Debug(lgm.CloseDB, lga.Handle, d.src.Handle)

	// We don't need to explicitly invoke c.impl.Close
	// because that's already been added to c.cleanup.
	return d.clnup.Run()
}
