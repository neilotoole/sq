// Package json implements the sq driver for JSON. There are three
// supported types:
// - JSON: plain old JSON
// - JSONA: JSON Array, where each record is an array of JSON values on its own line.
// - JSONL: JSON Lines, where each record a JSON object on its own line.
package json

import (
	"context"
	"database/sql"
	"io"
	"log/slog"

	"github.com/neilotoole/sq/libsq/files"

	"github.com/neilotoole/sq/libsq/source/location"

	"github.com/neilotoole/sq/libsq/core/cleanup"
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

// Provider implements driver.Provider.
type Provider struct {
	Log      *slog.Logger
	Ingester driver.GripOpenIngester
	Files    *files.Files
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	var ingestFn ingestFunc

	switch typ { //nolint:exhaustive
	case drivertype.JSON:
		ingestFn = ingestJSON
	case drivertype.JSONA:
		ingestFn = ingestJSONA
	case drivertype.JSONL:
		ingestFn = ingestJSONL
	default:
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{
		typ:      typ,
		ingester: d.Ingester,
		files:    d.Files,
		ingestFn: ingestFn,
	}, nil
}

// Driver implements driver.Driver.
type driveri struct {
	typ      drivertype.Type
	ingestFn ingestFunc
	ingester driver.GripOpenIngester
	files    *files.Files
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	md := driver.Metadata{Type: d.typ, Monotable: true}

	switch d.typ { //nolint:exhaustive
	case drivertype.JSON:
		md.Description = "JSON"
		md.Doc = "https://en.wikipedia.org/wiki/JSON"
	case drivertype.JSONA:
		md.Description = "JSON Array: LF-delimited JSON arrays"
		md.Doc = "https://en.wikipedia.org/wiki/JSON"
	case drivertype.JSONL:
		md.Description = "JSON Lines: LF-delimited JSON objects"
		md.Doc = "https://en.wikipedia.org/wiki/JSON_streaming#Line-delimited_JSON"
	}

	return md
}

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Grip, error) {
	log := lg.FromContext(ctx)
	log.Debug(lgm.OpenSrc, lga.Src, src)

	g := &grip{
		log:   log,
		src:   src,
		clnup: cleanup.New(),
		files: d.files,
	}

	allowCache := driver.OptIngestCache.Get(options.FromContext(ctx))

	ingestFn := func(ctx context.Context, destGrip driver.Grip) error {
		job := ingestJob{
			fromSrc: src,
			newRdrFn: func(ctx context.Context) (io.ReadCloser, error) {
				log.Debug("JSON ingest job newRdrFn", lga.Src, src)
				return d.files.NewReader(ctx, src, false)
			},
			destGrip:   destGrip,
			sampleSize: driver.OptIngestSampleSize.Get(src.Options),
			flatten:    true,
		}

		return d.ingestFn(ctx, job)
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
	clnup *cleanup.Cleanup
	files *files.Files
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

	md.Name, err = location.Filename(g.src.Location)
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

	return errz.Append(g.impl.Close(), g.clnup.Run())
}
