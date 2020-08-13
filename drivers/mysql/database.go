package mysql

import (
	"context"
	"database/sql"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
)

// database implements driver.Database.
type database struct {
	log  lg.Log
	db   *sql.DB
	src  *source.Source
	drvr *driveri
}

func (d *database) DB() *sql.DB {
	return d.db
}

func (d *database) SQLDriver() driver.SQLDriver {
	return d.drvr
}

func (d *database) Source() *source.Source {
	return d.src
}

func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	srcMeta, err := d.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return source.TableFromSourceMetadata(srcMeta, tblName)
}

func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	return getSourceMetadata(ctx, d.log, d.src, d.db)
}

func (d *database) Close() error {
	d.log.Debugf("Close database: %s", d.src)
	return errz.Err(d.db.Close())
}
