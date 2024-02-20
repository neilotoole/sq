// Package mdcache contains a [Cache] that caches source metadata.
package mdcache

import (
	"context"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/oncecache"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"golang.org/x/sync/errgroup"
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
}

// New returns a new [Cache].
func New(_ context.Context, coll *source.Collection, grips *driver.Grips) (c *Cache) {
	c = &Cache{
		coll:     coll,
		grips:    grips,
		tblMeta:  oncecache.New[source.Table, *metadata.Table](c.fetchTableMeta),
		srcMeta:  oncecache.New[string, *metadata.Source](c.fetchSourceMeta),
		tblNames: oncecache.New[string, []string](c.fetchTableNames),
	}

	return c
}

// Close closes the cache.
func (c *Cache) Close() error {
	// FIXME: Probably need to add a method oncecache.Cache.Close().
	// Unlike oncecache.Cache.Clear, it wouldn't send out notification signals.
	return nil
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

// TableMetaPair returns the [metadata.Table] pair for tbl1 and tbl2. The
// returned values are the internal cache entries, so the caller MUST NOT modify
// them. Use [metadata.Table.Clone] if necessary.
func (c *Cache) TableMetaPair(ctx context.Context, tbl1, tbl2 source.Table) (md1, md2 *metadata.Table, err error) {
	has1, has2 := c.tblMeta.Has(tbl1), c.tblMeta.Has(tbl2)

	if has1 || has2 {
		// We've got at least one of the pair, so there's no need for parallel
		// fetching, as one of them will come quickly from the Cache, and the other
		// will do the long fetch from the DB.
		var err1, err2 error
		md1, err1 = c.tblMeta.Get(ctx, tbl1)
		md2, err2 = c.tblMeta.Get(ctx, tbl2)
		return md1, md2, errz.Combine(err1, err2)
	}

	// We've got neither. Fetch both in parallel.
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var mdErr error
		md1, mdErr = c.tblMeta.Get(gCtx, tbl1)
		return mdErr
	})
	g.Go(func() error {
		var mdErr error
		md2, mdErr = c.tblMeta.Get(gCtx, tbl2)
		return mdErr
	})
	if err = g.Wait(); err != nil {
		return nil, nil, err
	}
	return md1, md2, err
}

// SourceMetaPair returns the [metadata.Source] pair for tbl1 and tbl2. The
// returned values are the internal cache entries, so the caller MUST NOT modify
// them. Use [metadata.Source.Clone] if necessary.
func (c *Cache) SourceMetaPair(ctx context.Context, src1, src2 *source.Source) (md1, md2 *metadata.Source, err error) {
	has1, has2 := c.srcMeta.Has(src1.Handle), c.srcMeta.Has(src2.Handle)

	if has1 || has2 {
		// We've got at least one of the pair, so there's no need for parallel
		// fetching, as one of them will come quickly from the Cache, and the other
		// will do the long fetch from the DB.
		var err1, err2 error
		md1, err1 = c.srcMeta.Get(ctx, src1.Handle)
		md2, err2 = c.srcMeta.Get(ctx, src2.Handle)
		return md1, md2, errz.Combine(err1, err2)
	}

	// We've got neither. Fetch both in parallel.
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var mdErr error
		md1, mdErr = c.srcMeta.Get(gCtx, src1.Handle)
		return mdErr
	})
	g.Go(func() error {
		var mdErr error
		md2, mdErr = c.srcMeta.Get(gCtx, src2.Handle)
		return mdErr
	})
	if err = g.Wait(); err != nil {
		return nil, nil, err
	}
	return md1, md2, err
}

// TableNamesPair returns the list of tables for tbl1 and tbl2. The returned
// values are the internal cache entries, so the caller MUST NOT modify them.
func (c *Cache) TableNamesPair(ctx context.Context, src1, src2 *source.Source) (tbls1, tbls2 []string, err error) {
	has1, has2 := c.tblNames.Has(src1.Handle), c.srcMeta.Has(src2.Handle)

	if has1 || has2 {
		// We've got at least one of the pair, so there's no need for parallel
		// fetching, as one of them will come quickly from the Cache, and the other
		// will do the long fetch from the DB.
		var err1, err2 error
		tbls1, err1 = c.tblNames.Get(ctx, src1.Handle)
		tbls2, err2 = c.tblNames.Get(ctx, src2.Handle)
		return tbls1, tbls2, errz.Combine(err1, err2)
	}

	// We've got neither. Fetch both in parallel.
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var mdErr error
		tbls1, mdErr = c.tblNames.Get(gCtx, src1.Handle)
		return mdErr
	})
	g.Go(func() error {
		var mdErr error
		tbls2, mdErr = c.tblNames.Get(gCtx, src2.Handle)
		return mdErr
	})
	if err = g.Wait(); err != nil {
		return nil, nil, err
	}
	return tbls1, tbls2, err
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
