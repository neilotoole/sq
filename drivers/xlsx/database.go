package xlsx

import (
	"context"
	"database/sql"
	"sync"

	"github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"golang.org/x/exp/slog"
)

// database implements driver.Database. It implements a deferred ingest
// of the Excel data.
type database struct {
	// REVISIT: do we need database.log, or can we use lg.FromContext?
	log *slog.Logger

	src       *source.Source
	files     *source.Files
	scratchDB driver.Database
	clnup     *cleanup.Cleanup

	mu         sync.Mutex
	ingestOnce sync.Once
	ingestErr  error

	// ingestSheetNames is the list of sheet names to ingest. When empty,
	// all sheets should be ingested. The key use of ingestSheetNames
	// is with TableMetadata, so that only the relevant table is ingested.
	ingestSheetNames []string
}

// checkIngest performs data ingestion if not already done.
func (d *database) checkIngest(ctx context.Context) error {
	d.ingestOnce.Do(func() {
		d.ingestErr = d.doIngest(ctx, d.ingestSheetNames)
	})

	return d.ingestErr
}

// doIngest performs data ingest. It must only be invoked from checkIngest.
func (d *database) doIngest(ctx context.Context, includeSheetNames []string) error {
	log := lg.FromContext(ctx)

	// Because of the deferred ingest mechanism, we need to ensure that
	// the context being passed down the stack (in particular to ingestXLSX)
	// has the source's options on it.
	ctx = options.NewContext(ctx, options.Merge(options.FromContext(ctx), d.src.Options))

	r, err := d.files.Open(d.src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	xlFile, err := excelize.OpenReader(r)
	if err != nil {
		return err
	}

	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, xlFile)

	err = ingestXLSX(ctx, d.src, d.scratchDB, xlFile, includeSheetNames)
	if err != nil {
		lg.WarnIfError(d.log, lgm.CloseDB, d.clnup.Run())
		return err
	}
	return err
}

// DB implements driver.Database.
func (d *database) DB(ctx context.Context) (*sql.DB, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.checkIngest(ctx); err != nil {
		return nil, err
	}

	return d.scratchDB.DB(ctx)
}

// SQLDriver implements driver.Database.
func (d *database) SQLDriver() driver.SQLDriver {
	return d.scratchDB.SQLDriver()
}

// Source implements driver.Database.
func (d *database) Source() *source.Source {
	return d.src
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context, noSchema bool) (*source.Metadata, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.checkIngest(ctx); err != nil {
		return nil, err
	}

	md, err := d.scratchDB.SourceMetadata(ctx, noSchema)
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
	d.mu.Lock()
	defer d.mu.Unlock()

	d.ingestSheetNames = []string{tblName}
	if err := d.checkIngest(ctx); err != nil {
		return nil, err
	}

	return d.scratchDB.TableMetadata(ctx, tblName)
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debug(lgm.CloseDB, lga.Handle, d.src.Handle)

	// No need to explicitly invoke c.scratchDB.Close because
	// that's already added to c.clnup
	return d.clnup.Run()
}
