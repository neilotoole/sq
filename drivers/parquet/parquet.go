// Package parquet implements the sq driver for Apache Parquet files.
// Parquet is a columnar binary format; this driver delegates reads to an
// in-memory DuckDB grip via the bundled "parquet" and "httpfs" extensions.
package parquet

import (
	"context"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/files"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// Provider implements driver.Provider.
type Provider struct {
	Log      *slog.Logger
	Registry *driver.Registry
	Files    *files.Files
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	if typ != drivertype.Parquet {
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}
	return &driveri{
		log:      p.Log,
		registry: p.Registry,
		files:    p.Files,
	}, nil
}

// driveri implements driver.Driver for Parquet files.
type driveri struct {
	log      *slog.Logger
	registry *driver.Registry
	files    *files.Files
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        drivertype.Parquet,
		Description: "Apache Parquet",
		Doc:         "https://parquet.apache.org",
		Monotable:   true,
	}
}

// Open implements driver.Driver. Real implementation lands in a later task.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)
	return nil, errz.New("parquet: Open not yet implemented")
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	d.log.Debug("Validating source", lga.Src, src)
	if src.Type != drivertype.Parquet {
		return nil, errz.Errorf("expected driver type {%s} but got {%s}",
			drivertype.Parquet, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	return d.files.Ping(ctx, src)
}

// errw wraps err with the package's standard boundary prefix. Errors crossing
// from DuckDB or the filesystem into sq go through here so the stack trace
// anchors at the parquet-side caller.
func errw(err error) error {
	return errz.Wrap(err, "parquet")
}
