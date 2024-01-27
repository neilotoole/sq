package sqlite3

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// grip implements driver.Grip.
type grip struct {
	log  *slog.Logger
	db   *sql.DB
	src  *source.Source
	drvr *driveri

	// closeOnce and closeErr are used to ensure that Close is only called once.
	// This is particularly relevant to sqlite, as calling Close multiple times
	// can cause problems on Windows.
	closeOnce sync.Once
	closeErr  error
}

// DB implements driver.Grip.
func (g *grip) DB(context.Context) (*sql.DB, error) {
	return g.db, nil
}

// SQLDriver implements driver.Grip.
func (g *grip) SQLDriver() driver.SQLDriver {
	return g.drvr
}

// Source implements driver.Grip.
func (g *grip) Source() *source.Source {
	return g.src
}

// TableMetadata implements driver.Grip.
func (g *grip) TableMetadata(ctx context.Context, tblName string) (*metadata.Table, error) {
	db, err := g.DB(ctx)
	if err != nil {
		return nil, err
	}

	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+"."+tblName+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	return getTableMetadata(ctx, db, tblName)
}

// SourceMetadata implements driver.Grip.
func (g *grip) SourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	// https://stackoverflow.com/questions/9646353/how-to-find-sqlite-database-file-version

	bar := progress.FromContext(ctx).NewUnitCounter(g.Source().Handle+": read schema", "item")
	defer bar.Stop()
	ctx = progress.NewBarContext(ctx, bar)

	start := time.Now()
	md, err := g.getSourceMetadata(ctx, noSchema)
	if err != nil {
		return nil, err
	}

	lg.FromContext(ctx).Debug("Read source metadata", lga.Src, g.src, lga.Elapsed, time.Since(start))
	return md, nil
}

// SourceMetadata implements driver.Grip.
func (g *grip) getSourceMetadata(ctx context.Context, noSchema bool) (*metadata.Source, error) {
	// https://stackoverflow.com/questions/9646353/how-to-find-sqlite-database-file-version

	md := &metadata.Source{Handle: g.src.Handle, Driver: drivertype.SQLite, DBDriver: dbDrvr}

	dsn, err := PathFromLocation(g.src)
	if err != nil {
		return nil, err
	}

	const q = "SELECT sqlite_version(), (SELECT name FROM pragma_database_list ORDER BY seq limit 1);"

	err = g.db.QueryRowContext(ctx, q).Scan(&md.DBVersion, &md.Schema)
	if err != nil {
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)

	md.DBProduct = "SQLite3 v" + md.DBVersion

	fi, err := os.Stat(dsn)
	if err != nil {
		return nil, errw(err)
	}

	md.Size = fi.Size()
	md.Name = fi.Name()
	md.FQName = fi.Name() + "." + md.Schema
	// SQLite doesn't support catalog, but we conventionally set it to "default"
	md.Catalog = "default"
	md.Location = g.src.Location

	md.DBProperties, err = getDBProperties(ctx, g.db)
	if err != nil {
		return nil, err
	}

	if noSchema {
		return md, nil
	}

	md.Tables, err = getAllTableMetadata(ctx, g.db, md.Schema)
	if err != nil {
		return nil, err
	}

	for _, tbl := range md.Tables {
		if tbl.TableType == sqlz.TableTypeTable {
			md.TableCount++
		} else if tbl.TableType == sqlz.TableTypeView {
			md.ViewCount++
		}
	}

	return md, nil
}

// Close implements driver.Grip. Subsequent calls to Close are no-op and
// return the same error.
func (g *grip) Close() error {
	g.closeOnce.Do(func() {
		g.log.Debug(lgm.CloseDB, lga.Handle, g.src.Handle)
		g.closeErr = errw(g.db.Close())
	})

	return g.closeErr
}
