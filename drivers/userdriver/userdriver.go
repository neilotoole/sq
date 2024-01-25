// Package userdriver implements the "user-driver" functionality
// that allows users to define source driver types declaratively.
// Note pkg userdriver itself is the framework: an actual
// implementation for each genre (such as XML) must be defined
// separately as in the "xmlud" sub-package.
package userdriver

import (
	"context"
	"io"
	"log/slog"

	"github.com/neilotoole/sq/libsq/files"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// IngestFunc is a function that can ingest
// data (as defined in def) to destGrip.
type IngestFunc func(ctx context.Context, def *DriverDef, data io.Reader, destGrip driver.Grip) error

// Provider implements driver.Provider for a DriverDef.
type Provider struct {
	Log       *slog.Logger
	DriverDef *DriverDef
	Ingester  driver.GripOpenIngester
	Files     *files.Files
	IngestFn  IngestFunc
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.Type(p.DriverDef.Name) {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{
		log:      p.Log,
		typ:      typ,
		def:      p.DriverDef,
		ingester: p.Ingester,
		ingestFn: p.IngestFn,
		files:    p.Files,
	}, nil
}

// Detectors returns funcs that can detect the driver type.
func (p *Provider) Detectors() []files.DriverDetectFunc {
	// TODO: it should be possible to return type detectors that
	//  can detect based upon the DriverDef. So, as of right
	//  now these detectors do nothing.
	return []files.DriverDetectFunc{}
}

// Driver implements driver.Driver.
type driveri struct {
	log      *slog.Logger
	typ      drivertype.Type
	def      *DriverDef
	files    *files.Files
	ingester driver.GripOpenIngester
	ingestFn IngestFunc
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.Type(d.def.Name),
		Description: d.def.Title,
		Doc:         d.def.Doc,
		UserDefined: true,
	}
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	log := lg.FromContext(ctx).With(lga.Src, src)
	log.Debug(lgm.OpenSrc)

	g := &grip{
		log: d.log,
		src: src,
	}

	allowCache := driver.OptIngestCache.Get(options.FromContext(ctx))

	ingestFn := func(ctx context.Context, destGrip driver.Grip) error {
		r, err := d.files.NewReader(ctx, src, false)
		if err != nil {
			return err
		}
		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)
		return d.ingestFn(ctx, d.def, r, destGrip)
	}

	var err error
	if g.impl, err = d.ingester.OpenIngest(ctx, src, allowCache, ingestFn); err != nil {
		return nil, err
	}
	return g, nil
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
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	return d.files.Ping(ctx, src)
}
