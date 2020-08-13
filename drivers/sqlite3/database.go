package sqlite3

import (
	"context"
	"database/sql"
	"os"

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

// DB implements driver.Database.
func (d *database) DB() *sql.DB {
	return d.db
}

// SQLDriver implements driver.Database.
func (d *database) SQLDriver() driver.SQLDriver {
	return d.drvr
}

// Source implements driver.Database.
func (d *database) Source() *source.Source {
	return d.src
}

// TableMetadata implements driver.Database.
func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	return tableMetadata(ctx, d.log, d.DB(), tblName)
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	// https://stackoverflow.com/questions/9646353/how-to-find-sqlite-database-file-version

	meta := &source.Metadata{Handle: d.src.Handle, SourceType: Type, DBDriverType: dbDrvr}

	dsn, err := PathFromLocation(d.src)
	if err != nil {
		return nil, err
	}

	err = d.DB().QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&meta.DBVersion)
	if err != nil {
		return nil, errz.Err(err)
	}
	meta.DBProduct = "SQLite3 v" + meta.DBVersion

	var schemaName string // typically "main"
	err = d.DB().QueryRowContext(ctx, "SELECT name FROM pragma_database_list ORDER BY seq LIMIT 1").Scan(&schemaName)
	if err != nil {
		return nil, errz.Err(err)
	}

	fi, err := os.Stat(dsn)
	if err != nil {
		return nil, errz.Err(err)
	}

	meta.Size = fi.Size()
	meta.Name = fi.Name()
	meta.FQName = fi.Name() + "/" + schemaName
	meta.Location = d.src.Location

	meta.Tables, err = getAllTblMeta(ctx, d.log, d.db)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debugf("Close database: %s", d.src)

	return errz.Err(d.db.Close())
}
