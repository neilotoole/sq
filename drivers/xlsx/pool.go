package xlsx

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"

	excelize "github.com/xuri/excelize/v2"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// pool implements driver.Pool. It implements a deferred ingest
// of the Excel data.
type pool struct {
	// REVISIT: do we need pool.log, or can we use lg.FromContext?
	log *slog.Logger

	src         *source.Source
	files       *source.Files
	scratcher   driver.ScratchPoolOpener
	scratchPool driver.Pool
	clnup       *cleanup.Cleanup

	mu         sync.Mutex
	ingestOnce sync.Once
	ingestErr  error

	// ingestSheetNames is the list of sheet names to ingest. When empty,
	// all sheets should be ingested. The key use of ingestSheetNames
	// is with pool.TableMetadata, so that only the relevant table is ingested.
	//
	// FIXME: Verify how ingestSheetNames interacts with the deferred
	// ingest and caching mechanisms. E.g. if we ingest a single sheet,
	// and then later ingest the entire workbook, will the cache DB be
	// accurate?
	// ACTUALLY: We can get rid of this entirely, because with caching,
	// there's no longer really a problem.
	ingestSheetNames []string
}

// checkIngest performs data ingestion if not already done.
func (p *pool) checkIngest(ctx context.Context) error {
	p.ingestOnce.Do(func() {
		p.ingestErr = p.doIngest(ctx, p.ingestSheetNames)
	})

	return p.ingestErr
}

// doIngest performs data ingest. It must only be invoked from checkIngest.
func (p *pool) doIngest(ctx context.Context, includeSheetNames []string) error {
	log := lg.FromContext(ctx)

	// Because of the deferred ingest mechanism, we need to ensure that
	// the context being passed down the stack (in particular to ingestXLSX)
	// has the source's options on it.
	ctx = options.NewContext(ctx, options.Merge(options.FromContext(ctx), p.src.Options))
	allowCache := driver.OptIngestCache.Get(options.FromContext(ctx))

	ingestFn := func(ctx context.Context, destPool driver.Pool) error {
		log.Debug("Ingest XLSX", lga.Src, p.src)
		//openFn := p.files.OpenFunc(p.src)
		r, err := p.files.Open(ctx, p.src)
		if err != nil {
			return err
		}
		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

		xfile, err := excelize.OpenReader(r, excelize.Options{RawCellValue: false})
		if err != nil {
			return err
		}

		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, xfile)

		if err = ingestXLSX(ctx, p.src, destPool, xfile, includeSheetNames); err != nil {
			lg.WarnIfError(log, lgm.CloseDB, p.clnup.Run())
			return err
		}
		return nil
	}

	var err error
	p.scratchPool, err = p.scratcher.OpenIngest(ctx, p.src, ingestFn, allowCache)
	return err
}

// DB implements driver.Pool.
func (p *pool) DB(ctx context.Context) (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.checkIngest(ctx); err != nil {
		return nil, err
	}

	return p.scratchPool.DB(ctx)
}

// SQLDriver implements driver.Pool.
func (p *pool) SQLDriver() driver.SQLDriver {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.checkIngest(ctx); err != nil {
		return nil, err
	}
	return p.scratchPool.SQLDriver()
}

// Source implements driver.Pool.
func (p *pool) Source() *source.Source {
	return p.src
}

// SourceMetadata implements driver.Pool.
func (p *pool) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.checkIngest(ctx); err != nil {
		return nil, err
	}

	md, err := p.scratchPool.SourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	md.Handle = p.src.Handle
	md.Driver = Type
	md.Location = p.src.Location
	if md.Name, err = source.LocationFileName(p.src); err != nil {
		return nil, err
	}
	md.FQName = md.Name

	if md.Size, err = p.files.Size(ctx, p.src); err != nil {
		return nil, err
	}

	return md, nil
}

// TableMetadata implements driver.Pool.
func (p *pool) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ingestSheetNames = []string{tblName}
	if err := p.checkIngest(ctx); err != nil {
		return nil, err
	}

	return p.scratchPool.TableMetadata(ctx, tblName)
}

// Close implements driver.Pool.
func (p *pool) Close() error {
	p.log.Debug(lgm.CloseDB, lga.Handle, p.src.Handle)

	// No need to explicitly invoke c.scratchPool.Close because
	// that's already added to c.clnup
	return p.clnup.Run()
}
