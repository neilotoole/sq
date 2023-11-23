// Package json implements the sq driver for JSON. There are three
// supported types:
// - JSON: plain old JSON
// - JSONA: JSON Array, where each record is an array of JSON values on its own line.
// - JSONL: JSON Lines, where each record a JSON object on its own line.
package json

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

const (
	// TypeJSON is the plain-old JSON driver type.
	TypeJSON = drivertype.Type("json")

	// TypeJSONA is the JSON Array driver type.
	TypeJSONA = drivertype.Type("jsona")

	// TypeJSONL is the JSON Lines driver type.
	TypeJSONL = drivertype.Type("jsonl")
)

// Provider implements driver.Provider.
type Provider struct {
	Log       *slog.Logger
	Scratcher driver.ScratchPoolOpener
	Files     *source.Files
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ drivertype.Type) (driver.Driver, error) {
	var importFn importFunc

	switch typ { //nolint:exhaustive
	case TypeJSON:
		importFn = importJSON
	case TypeJSONA:
		importFn = importJSONA
	case TypeJSONL:
		importFn = importJSONL
	default:
		return nil, errz.Errorf("unsupported driver type {%s}", typ)
	}

	return &driveri{
		log:       d.Log,
		typ:       typ,
		scratcher: d.Scratcher,
		files:     d.Files,
		importFn:  importFn,
	}, nil
}

// Driver implements driver.Driver.
type driveri struct {
	log       *slog.Logger
	typ       drivertype.Type
	importFn  importFunc
	scratcher driver.ScratchPoolOpener
	files     *source.Files
}

// DriverMetadata implements driver.Driver.
func (d *driveri) DriverMetadata() driver.Metadata {
	md := driver.Metadata{Type: d.typ, Monotable: true}

	switch d.typ { //nolint:exhaustive
	case TypeJSON:
		md.Description = "JSON"
		md.Doc = "https://en.wikipedia.org/wiki/JSON"
	case TypeJSONA:
		md.Description = "JSON Array: LF-delimited JSON arrays"
		md.Doc = "https://en.wikipedia.org/wiki/JSON"
	case TypeJSONL:
		md.Description = "JSON Lines: LF-delimited JSON objects"
		md.Doc = "https://en.wikipedia.org/wiki/JSON_streaming#Line-delimited_JSON"
	}

	return md
}

// Open implements driver.PoolOpener.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Pool, error) {
	lg.FromContext(ctx).Debug(lgm.OpenSrc, lga.Src, src)

	p := &pool{log: d.log, src: src, clnup: cleanup.New(), files: d.files}

	r, err := d.files.Open(src)
	if err != nil {
		return nil, err
	}

	p.impl, err = d.scratcher.OpenScratchFor(ctx, src)
	if err != nil {
		lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)
		lg.WarnIfFuncError(d.log, lgm.CloseDB, p.clnup.Run)
		return nil, err
	}

	job := importJob{
		fromSrc:    src,
		openFn:     d.files.OpenFunc(src),
		destPool:   p.impl,
		sampleSize: driver.OptIngestSampleSize.Get(src.Options),
		flatten:    true,
	}

	err = d.importFn(ctx, job)
	if err != nil {
		lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)
		lg.WarnIfFuncError(d.log, lgm.CloseDB, p.clnup.Run)
		return nil, err
	}

	err = r.Close()
	if err != nil {
		return nil, err
	}

	return p, nil
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
	d.log.Debug("Ping source", lga.Src, src)

	r, err := d.files.Open(src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	return nil
}

// pool implements driver.Pool.
type pool struct {
	log   *slog.Logger
	src   *source.Source
	impl  driver.Pool
	clnup *cleanup.Cleanup
	files *source.Files
}

// DB implements driver.Pool.
func (p *pool) DB(ctx context.Context) (*sql.DB, error) {
	return p.impl.DB(ctx)
}

// SQLDriver implements driver.Pool.
func (p *pool) SQLDriver() driver.SQLDriver {
	return p.impl.SQLDriver()
}

// Source implements driver.Pool.
func (p *pool) Source() *source.Source {
	return p.src
}

// TableMetadata implements driver.Pool.
func (p *pool) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	if tblName != source.MonotableName {
		return nil, errz.Errorf("table name should be %s for CSV/TSV etc., but got: %s",
			source.MonotableName, tblName)
	}

	srcMeta, err := p.SourceMetadata(ctx, false)
	if err != nil {
		return nil, err
	}

	// There will only ever be one table for CSV.
	return srcMeta.Tables[0], nil
}

// SourceMetadata implements driver.Pool.
func (p *pool) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	md, err := p.impl.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	md.Handle = p.src.Handle
	md.Location = p.src.Location
	md.Driver = p.src.Type

	md.Name, err = source.LocationFileName(p.src)
	if err != nil {
		return nil, err
	}

	md.Size, err = p.files.Size(p.src)
	if err != nil {
		return nil, err
	}

	md.FQName = md.Name
	return md, nil
}

// Close implements driver.Pool.
func (p *pool) Close() error {
	p.log.Debug(lgm.CloseDB, lga.Handle, p.src.Handle)

	return errz.Combine(p.impl.Close(), p.clnup.Run())
}
