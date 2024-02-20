package mdcache

import (
	"context"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/oncecache"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"golang.org/x/sync/errgroup"
)

type cache struct {
	ru       *run.Run
	tblMeta  *oncecache.Cache[source.Table, *metadata.Table]
	srcMeta  *oncecache.Cache[string, *metadata.Source]
	tblNames *oncecache.Cache[string, []string]
}

func newCache(ru *run.Run) (c *cache) {
	c = &cache{
		ru:       ru,
		tblMeta:  oncecache.New[source.Table, *metadata.Table](c.fetchTableMeta),
		srcMeta:  oncecache.New[string, *metadata.Source](c.fetchSourceMeta),
		tblNames: oncecache.New[string, []string](c.fetchTableNames),
	}

	return c
}

func (c *cache) fetchTableNames(ctx context.Context, handle string) ([]string, error) {
	src, err := c.ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, err
	}

	db, drvr, err := c.ru.DB(ctx, src)
	if err != nil {
		return nil, err
	}

	tbls, err := drvr.ListTableNames(ctx, db, "", true, true)
	if err != nil {
		if errz.Has[*driver.NotExistError](err) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	return tbls, err
}

func (c *cache) fetchTableMeta(ctx context.Context, tbl source.Table) (*metadata.Table, error) {
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
func (c *cache) fetchSourceMeta(ctx context.Context, handle string) (*metadata.Source, error) {
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

func (c *cache) gripForTable(ctx context.Context, tbl source.Table) (grip driver.Grip, err error) {
	src, err := c.ru.Config.Collection.Get(tbl.Handle)
	if err != nil {
		return nil, err
	}

	return c.ru.Grips.Open(ctx, src)
}
func (c *cache) gripForHandle(ctx context.Context, handle string) (grip driver.Grip, err error) {
	src, err := c.ru.Config.Collection.Get(handle)
	if err != nil {
		return nil, err
	}

	return c.ru.Grips.Open(ctx, src)
}

// getSource doesn't actually use the cache - the source comes from the config
// source collection - but the method is here for uniformity.
func (c *cache) getSource(handle string) *source.Source {
	src, _ := c.ru.Config.Collection.Get(handle)
	return src
}

// getTableMetaPair returns the [metadata.Table] for tbl1 and tbl2.
func (c *cache) getTableMetaPair(ctx context.Context, tbl1, tbl2 source.Table) (md1, md2 *metadata.Table, err error) {
	has1, has2 := c.tblMeta.Has(tbl1), c.tblMeta.Has(tbl2)

	if has1 || has2 {
		// We've got at least one of the pair, so there's no need for parallel
		// fetching, as one of them will come quickly from the cache, and the other
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

// fetchTableMeta returns the metadata.Table for table. If the table
// does not exist, {nil,nil} is returned.
//
// Deprecated: use cache.getTableMeta instead.
func fetchTableMeta(ctx context.Context, ru *run.Run, src *source.Source, table string) (*metadata.Table, error) {
	grip, err := ru.Grips.Open(ctx, src)
	if err != nil {
		return nil, err
	}
	md, err := grip.TableMetadata(ctx, table)
	if err != nil {
		if errz.Has[*driver.NotExistError](err) {
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}

	return md, nil
}

//// tableData encapsulates data about a table.
//type tableData struct {
//	tbl     source.Table
//	tblMeta *metadata.Table
//	src     *source.Source
//	srcMeta *metadata.Source
//}
//
//// String returns @handle.table.
//func (td *tableData) String() string {
//	return td.tbl.String()
//}
