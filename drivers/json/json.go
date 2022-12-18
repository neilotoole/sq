// Package json implements the sq driver for JSON. There are three
// supported types:
// - JSON: plain old JSON
// - JSONA: JSON Array, where each record is an array of JSON values on its own line.
// - JSONL: JSON Lines, where each record a JSON object on its own line.
package json

import (
	"context"
	"database/sql"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	// TypeJSON is the plain-old JSON driver type.
	TypeJSON = source.Type("json")

	// TypeJSONA is the JSON Array driver type.
	TypeJSONA = source.Type("jsona")

	// TypeJSONL is the JSON Lines driver type.
	TypeJSONL = source.Type("jsonl")
)

// Provider implements driver.Provider.
type Provider struct {
	Log       lg.Log
	Scratcher driver.ScratchDatabaseOpener
	Files     *source.Files
}

// DriverFor implements driver.Provider.
func (d *Provider) DriverFor(typ source.Type) (driver.Driver, error) {
	var importFn importFunc

	switch typ { //nolint:exhaustive
	case TypeJSON:
		importFn = importJSON
	case TypeJSONA:
		importFn = importJSONA
	case TypeJSONL:
		importFn = importJSONL
	default:
		return nil, errz.Errorf("unsupported driver type %q", typ)
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
	log       lg.Log
	typ       source.Type
	importFn  importFunc
	scratcher driver.ScratchDatabaseOpener
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

// Open implements driver.Driver.
func (d *driveri) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	dbase := &database{log: d.log, src: src, clnup: cleanup.New(), files: d.files}

	r, err := d.files.Open(src)
	if err != nil {
		return nil, err
	}

	dbase.impl, err = d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		d.log.WarnIfCloseError(r)
		d.log.WarnIfFuncError(dbase.clnup.Run)
		return nil, err
	}

	job := importJob{
		fromSrc:    src,
		openFn:     d.files.OpenFunc(src),
		destDB:     dbase.impl,
		sampleSize: driver.Tuning.SampleSize,
		flatten:    true, // TODO: Should come from src.Options
	}

	err = d.importFn(ctx, d.log, job)
	if err != nil {
		d.log.WarnIfCloseError(r)
		d.log.WarnIfFuncError(dbase.clnup.Run)
		return nil, err
	}

	err = r.Close()
	if err != nil {
		return nil, err
	}

	return dbase, nil
}

// Truncate implements driver.Driver.
func (d *driveri) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (int64, error) {
	return 0, errz.Errorf("truncate not supported for %s", d.DriverMetadata().Type)
}

// ValidateSource implements driver.Driver.
func (d *driveri) ValidateSource(src *source.Source) (*source.Source, error) {
	if src.Type != d.typ {
		return nil, errz.Errorf("expected source type %q but got %q", d.typ, src.Type)
	}

	return src, nil
}

// Ping implements driver.Driver.
func (d *driveri) Ping(ctx context.Context, src *source.Source) error {
	d.log.Debugf("driver %q attempting to ping %q", d.typ, src)

	r, err := d.files.Open(src)
	if err != nil {
		return err
	}
	defer d.log.WarnIfCloseError(r)

	return nil
}

// database implements driver.Database.
type database struct {
	log   lg.Log
	src   *source.Source
	impl  driver.Database
	clnup *cleanup.Cleanup
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
	md.SourceType = d.src.Type

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
	d.log.Debugf("Close database: %s", d.src)

	return errz.Combine(d.impl.Close(), d.clnup.Run())
}

var (
	_ source.TypeDetectFunc = DetectJSON
	_ source.TypeDetectFunc = DetectJSONA
	_ source.TypeDetectFunc = DetectJSONL
)
