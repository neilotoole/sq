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

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// ImportFunc is a function that can import
// data (as defined in def) to destDB.
type ImportFunc func(ctx context.Context, log lg.Log, def *DriverDef, data io.Reader, destDB driver.Database) error

// Provider implements driver.Provider for a DriverDef.
type Provider struct {
	Log       lg.Log
	DriverDef *DriverDef
	Scratcher driver.ScratchDatabaseOpener
	Files     *source.Files
	ImportFn  ImportFunc
}

// DriverFor implements driver.Provider.
func (p *Provider) DriverFor(typ source.Type) (driver.Driver, error) {
	if typ != source.Type(p.DriverDef.Name) {
		return nil, errz.Errorf("unsupported driver type %q", typ)
	}

	return &drvr{
		log:       p.Log,
		typ:       typ,
		def:       p.DriverDef,
		scratcher: p.Scratcher,
		importFn:  p.ImportFn,
		files:     p.Files,
	}, nil
}

// TypeDetectors returns funcs that can detect the source type.
func (p *Provider) TypeDetectors() []source.TypeDetectFunc {
	// TODO: it should be possible to return type detectors that
	//  can detect based upon the DriverDef. So, as of right
	//  now these detectors do nothing.
	return []source.TypeDetectFunc{}
}

// Driver implements driver.Driver.
type drvr struct {
	log       lg.Log
	typ       source.Type
	def       *DriverDef
	files     *source.Files
	scratcher driver.ScratchDatabaseOpener
	importFn  ImportFunc
}

// DriverMetadata implements driver.Driver.
func (d *drvr) DriverMetadata() driver.Metadata {
	return driver.Metadata{
		Type:        source.Type(d.def.Name),
		Description: d.def.Title,
		Doc:         d.def.Doc,
		UserDefined: true,
	}
}

// Open implements driver.Driver.
func (d *drvr) Open(ctx context.Context, src *source.Source) (driver.Database, error) {
	clnup := cleanup.New()

	r, err := d.files.Open(src)
	if err != nil {
		return nil, err
	}

	defer d.log.WarnIfCloseError(r)

	scratchDB, err := d.scratcher.OpenScratch(ctx, src.Handle)
	if err != nil {
		return nil, err
	}
	clnup.AddE(scratchDB.Close)

	err = d.importFn(ctx, d.log, d.def, r, scratchDB)
	if err != nil {
		d.log.WarnIfFuncError(clnup.Run)
		return nil, errz.Wrap(err, d.def.Name)
	}

	return &database{log: d.log, src: src, impl: scratchDB, clnup: clnup}, nil
}

// Truncate implements driver.Driver.
func (d *drvr) Truncate(ctx context.Context, src *source.Source, tbl string, reset bool) (int64, error) {
	return 0, errz.Errorf("truncate not supported for %s", d.DriverMetadata().Type)
}

// ValidateSource implements driver.Driver.
func (d *drvr) ValidateSource(src *source.Source) (*source.Source, error) {
	d.log.Debugf("validating source: %q", src.RedactedLocation())
	if string(src.Type) != d.def.Name {
		return nil, errz.Errorf("expected source type %q but got %q", d.def.Name, src.Type)
	}
	return src, nil
}

// Ping implements driver.Driver.
func (d *drvr) Ping(ctx context.Context, src *source.Source) error {
	d.log.Debugf("driver %q attempting to ping %q", d.typ, src)

	r, err := d.files.Open(src)
	if err != nil {
		return err
	}

	// TODO: possibly do something more useful than just
	//  getting the reader?

	return r.Close()
}

// database implements driver.Database.
type database struct {
	log  lg.Log
	src  *source.Source
	impl driver.Database

	// clnup will ultimately invoke impl.Close to dispose of
	// the scratch DB.
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

// TableMetadata implements driver.Database.
func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	return d.impl.TableMetadata(ctx, tblName)
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	meta, err := d.impl.SourceMetadata(ctx)
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

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debugf("Close database: %s", d.src)

	// We don't need to explicitly invoke c.impl.Close
	// because that's already been added to c.cleanup.
	return d.clnup.Run()
}
