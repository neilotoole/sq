package xlsx

import (
	"context"
	"database/sql"
	"io"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/tealeg/xlsx/v2"

	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"golang.org/x/exp/slog"
)

// database implements driver.Database.
type database struct {
	log       *slog.Logger
	src       *source.Source
	files     *source.Files
	scratchDB driver.Database
	clnup     *cleanup.Cleanup
	ingested  bool
	// ingestOnce *sync.Once
	// ingestErr  error
}

func (d *database) doIngest(ctx context.Context) error {
	r, err := d.files.Open(d.src)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(d.log, lgm.CloseFileReader, r)

	b, err := io.ReadAll(r)
	if err != nil {
		return errz.Err(err)
	}

	xlFile, err := xlsx.OpenBinary(b)
	if err != nil {
		return err
	}

	//scratchDB, err := d.scratcher.OpenScratch(ctx, src.Handle)
	//if err != nil {
	//	return nil, err
	//}

	// clnup := cleanup.New()
	// clnup.AddE(scratchDB.Close)

	// REVISIT: Can we defer ingest?
	err = ingest(ctx, d.src, d.scratchDB, xlFile, nil)
	//if err != nil {
	//	lg.WarnIfError(d.log, lgm.CloseDB, clnup.Run())
	//	return err
	//}
	return err
}

// DB implements driver.Database.
func (d *database) DB() *sql.DB {
	return d.scratchDB.DB()
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
