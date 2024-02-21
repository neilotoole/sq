// Package mdcache contains a [Cache] that caches source metadata.
package mdcache

import (
	"context"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/oncecache"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// Cache caches source metadata. Note that the accessor methods return the
// actual internal cache values, not copies, so the caller MUST NOT modify
// the returned values.
type Cache struct {
	coll     *source.Collection
	grips    *driver.Grips
	tblMeta  *oncecache.Cache[source.Table, *metadata.Table]
	srcMeta  *oncecache.Cache[string, *metadata.Source]
	tblNames *oncecache.Cache[string, []string]
	dbProps  *oncecache.Cache[string, map[string]any]
}

// New returns a new [Cache]. If log is non-nil, it will be used for logging
// cache events.
func New(log *slog.Logger, coll *source.Collection, grips *driver.Grips) *Cache {
	c := &Cache{coll: coll, grips: grips}

	c.tblMeta = oncecache.New[source.Table, *metadata.Table](
		c.fetchTableMeta,
		oncecache.Name("mdcache.tblMeta"),
		oncecache.Log(log, slog.LevelDebug),
	)

	c.srcMeta = oncecache.New[string, *metadata.Source](
		c.fetchSourceMeta,
		oncecache.Name("mdcache.srcMeta"),
		oncecache.Log(log, slog.LevelDebug),
	)
	c.tblNames = oncecache.New[string, []string](
		c.fetchTableNames,
		oncecache.Name("mdcache.tblNames"),
		oncecache.Log(log, slog.LevelDebug),
	)

	c.dbProps = oncecache.New[string, map[string]any](
		c.fetchDBProps,
		oncecache.Name("mdcache.dbProps"),
		oncecache.Log(log, slog.LevelDebug),
	)

	return c
}

// Close closes the cache.
func (c *Cache) Close() error {
	if c == nil {
		return nil
	}

	return errz.Combine(
		c.tblNames.Close(),
		c.tblMeta.Close(),
		c.srcMeta.Close(),
		c.dbProps.Close(),
	)
}

// TableMeta returns the metadata for tbl. The returned value is the internal
// cache entry, so the caller MUST NOT modify it. Use [metadata.Table.Clone]
// if necessary.
func (c *Cache) TableMeta(ctx context.Context, tbl source.Table) (*metadata.Table, error) {
	return c.tblMeta.Get(ctx, tbl)
}

// SourceMeta returns the metadata for the source. The returned value is the
// internal cache entry, so the caller MUST NOT modify it. Use
// [metadata.Source.Clone] if necessary.
func (c *Cache) SourceMeta(ctx context.Context, handle string) (*metadata.Source, error) {
	return c.srcMeta.Get(ctx, handle)
}

// TableNames returns the table names for the source. The returned value is the
// internal cache entry, so the caller MUST NOT modify it.
func (c *Cache) TableNames(ctx context.Context, handle string) ([]string, error) {
	return c.tblNames.Get(ctx, handle)
}

// DBProperties returns the DB properties for the source. The returned value is the
// internal cache entry, so the caller MUST NOT modify it.
func (c *Cache) DBProperties(ctx context.Context, handle string) (map[string]any, error) {
	return c.dbProps.Get(ctx, handle)
}

// TableMetaPair returns the [metadata.Table] pair for tbl1 and tbl2. The
// returned values are the internal cache entries, so the caller MUST NOT modify
// them. Use [metadata.Table.Clone] if necessary.
func (c *Cache) TableMetaPair(ctx context.Context, tbl1, tbl2 source.Table) (md1, md2 *metadata.Table, err error) {
	return getPair(ctx, c.tblMeta, tbl1, tbl2)
}

// SourceMetaPair returns the [metadata.Source] pair for tbl1 and tbl2. The
// returned values are the internal cache entries, so the caller MUST NOT modify
// them. Use [metadata.Source.Clone] if necessary.
func (c *Cache) SourceMetaPair(ctx context.Context, src1, src2 *source.Source) (md1, md2 *metadata.Source, err error) {
	return getPair(ctx, c.srcMeta, src1.Handle, src2.Handle)
}

// TableNamesPair returns the list of tables for tbl1 and tbl2. The returned
// values are the internal cache entries, so the caller MUST NOT modify them.
func (c *Cache) TableNamesPair(ctx context.Context, src1, src2 *source.Source) (tbls1, tbls2 []string, err error) {
	return getPair(ctx, c.tblNames, src1.Handle, src2.Handle)
}

// DBPropertiesPair returns the DB properties for src1 and src2. The returned
// values are the internal cache entries, so the caller MUST NOT modify them.
func (c *Cache) DBPropertiesPair(ctx context.Context,
	src1, src2 *source.Source,
) (dbp1, dbp2 map[string]any, err error) {
	return getPair(ctx, c.dbProps, src1.Handle, src2.Handle)
}

// db is a convenience method that gets the sqlz.DB and driver.SQLDriver for
// src.
func (c *Cache) db(ctx context.Context, src *source.Source) (sqlz.DB, driver.SQLDriver, error) {
	grip, err := c.grips.Open(ctx, src)
	if err != nil {
		return nil, nil, err
	}
	d, err := grip.DB(ctx)
	if err != nil {
		return nil, nil, err
	}

	return d, grip.SQLDriver(), nil
}

func (c *Cache) fetchDBProps(ctx context.Context, handle string) (map[string]any, error) {
	src, err := c.coll.Get(handle)
	if err != nil {
		return nil, err
	}

	db, drvr, err := c.db(ctx, src)
	if err != nil {
		return nil, err
	}

	dbProps, err := drvr.DBProperties(ctx, db)
	if err != nil {
		if errz.Has[*driver.NotExistError](err) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	return dbProps, err
}

func (c *Cache) fetchTableNames(ctx context.Context, handle string) ([]string, error) {
	src, err := c.coll.Get(handle)
	if err != nil {
		return nil, err
	}

	db, drvr, err := c.db(ctx, src)
	if err != nil {
		return nil, err
	}

	tbls, err := drvr.ListTableNames(ctx, db, "", true, true)
	if err != nil {
		if errz.Has[*driver.NotExistError](err) {
			return nil, nil
		}
		return nil, err
	}
	return tbls, err
}

func (c *Cache) fetchTableMeta(ctx context.Context, tbl source.Table) (*metadata.Table, error) {
	grip, err := c.gripForTable(ctx, tbl)
	if err != nil {
		return nil, err
	}

	md, err := grip.TableMetadata(ctx, tbl.Name)
	if err != nil {
		if errz.Has[*driver.NotExistError](err) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	return md, err
}

func (c *Cache) fetchSourceMeta(ctx context.Context, handle string) (*metadata.Source, error) {
	grip, err := c.gripForHandle(ctx, handle)
	if err != nil {
		return nil, err
	}

	md, err := grip.SourceMetadata(ctx, false)
	if err != nil {
		if errz.Has[*driver.NotExistError](err) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	return md, err
}

func (c *Cache) gripForTable(ctx context.Context, tbl source.Table) (grip driver.Grip, err error) {
	src, err := c.coll.Get(tbl.Handle)
	if err != nil {
		return nil, err
	}

	return c.grips.Open(ctx, src)
}

func (c *Cache) gripForHandle(ctx context.Context, handle string) (grip driver.Grip, err error) {
	src, err := c.coll.Get(handle)
	if err != nil {
		return nil, err
	}

	return c.grips.Open(ctx, src)
}

// getPair is a helper that fetches a pair of values from the cache. If both
// values are absent from the cache, they are fetched in parallel.
func getPair[K comparable, V any](ctx context.Context, c *oncecache.Cache[K, V],
	key1, key2 K,
) (val1, val2 V, err error) {
	has1, has2 := c.Has(key1), c.Has(key2)

	if has1 || has2 {
		// We've got at least one of the pair, so there's no need for parallel
		// fetching, as one of them will come quickly from the Cache, and the other
		// will do the long fetch from the DB.
		var err1, err2 error
		val1, err1 = c.Get(ctx, key1)
		val2, err2 = c.Get(ctx, key2)
		return val1, val2, errz.Combine(err1, err2)
	}

	// We've got neither. Fetch both in parallel.
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var gErr error
		val1, gErr = c.Get(gCtx, key1)
		return gErr
	})
	g.Go(func() error {
		var mdErr error
		val2, mdErr = c.Get(gCtx, key2)
		return mdErr
	})
	if err = g.Wait(); err != nil {
		return val1, val2, err
	}
	return val1, val2, err
}
