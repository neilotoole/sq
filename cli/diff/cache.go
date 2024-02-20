package diff

import (
	"context"
	"sync"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/upgrademu"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"golang.org/x/sync/errgroup"
)

type cache struct {
	// FIXME: Add ack for upgrademu
	// https://gist.github.com/sancar/d1663e90892cd12c839ae21841b79295
	upmu       upgrademu.RWMutex
	ru         *run.Run
	tblMeta    map[string]*cacheEntry[*metadata.Table]
	srcTblList map[string]*cacheEntry[[]string]
	srcMeta    map[string]*cacheEntry[*metadata.Source]
}

func (c *cache) newTableMetaCacheEntry(key string) *cacheEntry[*metadata.Table] {
	return &cacheEntry[*metadata.Table]{
		key: key,
		getter: func(ctx context.Context, key string) (*metadata.Table, error) {
			return fetchTableMeta(ctx, c.ru, src, table)
		},
	}
}

//
//type tableMetaEntry struct {
//	key     source.Table
//	once    sync.Once
//	tblMeta *metadata.Table
//	err     error
//}
//
//type srcTableListEntry struct {
//	key        string
//	once       sync.Once
//	tableNames []string
//	err        error
//}

//type srcMetaEntry struct {
//	mu      sync.RWMutex
//	key     string
//	srcMeta *metadata.Source
//}

type cacheEntry[K comparable, V any] struct {
	key K
	val V
	err error
	// getOnce sync.Once
	// setOnce sync.Once
	once sync.Once
	// getter  func(ctx context.Context, key K, setter func(val V, err error))
	getter2 func(ctx context.Context, key K) (val V, err error)
}

func (ce *cacheEntry[K, V]) set(val V, err error) {
	ce.once.Do(func() {
		ce.val = val
		ce.err = err
	})
}

//	func (ce *cacheEntry[K, V]) get(ctx context.Context) (V, error) {
//		ce.getOnce.Do(func() {
//			ce.getter(ctx, ce.key, ce.set)
//		})
//		return ce.val, ce.err
//	}
func (ce *cacheEntry[K, V]) get(ctx context.Context) (V, error) {
	ce.once.Do(func() {
		ce.val, ce.err = ce.getter2(ctx, ce.key)
		// ce.getter(ctx, ce.key, ce.set)
	})
	return ce.val, ce.err
}

func (c *cache) newSourceMetaCacheEntry(key string) *cacheEntry[string, *metadata.Source] {
	return &cacheEntry[string, *metadata.Source]{
		key: key,
		//getter: func(ctx context.Context, key string, setter func(val *metadata.Source, err error)) {
		//	_, srcMeta, err := fetchSourceMeta(ctx, c.ru, key)
		//	setter(srcMeta, err)
		//},
		getter2: func(ctx context.Context, key string) (val *metadata.Source, err error) {
			_, srcMeta, err := fetchSourceMeta(ctx, c.ru, key)
			return srcMeta, err
		},
	}
}

// getSource doesn't actually use the cache, the source comes from the config
// source collection, but the method is here for uniformity.
func (c *cache) getSource(handle string) *source.Source {
	src, _ := c.ru.Config.Collection.Get(handle)
	return src
}

// getTableMeta returns the [metadata.Table] for tbl. If the table does not
// exist, then (nil,nil) is returned, not an error.
//
// The caller must not mutate the returned [metadata.Table]. Use
// [metadata.Table.Clone] if necessary.
func (c *cache) getTableMeta(ctx context.Context, tbl source.Table) (*metadata.Table, error) {
	// FIXME: We probably need a lock per table.
	c.upmu.UpgradableRLock()

	entry, ok := c.tblMeta[tbl]
	if ok {
		c.upmu.UpgradableRUnlock()
		entry.mu.RLock()
		tblMeta := entry.tblMeta
		entry.mu.RUnlock()
		return tblMeta, nil
	}

	entry = &tableMetaEntry{key: tbl}
	c.upmu.UpgradeWLock()
	c.tblMeta[tbl] = entry

	c.upmu.Lock()
	defer c.upmu.Unlock()
	src, err := c.ru.Config.Collection.Get(tbl.Handle)
	if err != nil {
		return nil, err
	}

	grip, err := c.ru.Grips.Open(ctx, src)
	if err != nil {
		return nil, err
	}
	md, err = grip.TableMetadata(ctx, tbl.Name)
	if err != nil {
		if errz.Has[*driver.NotExistError](err) {
			// table does not exist
			c.tblMeta[tbl] = nil
			return nil, nil //nolint:nilnil
		}
		return nil, err
	}
	c.tblMeta[tbl] = md

	return md, nil
}

// getTableMetaPair returns the [metadata.Table] for tbl1 and tbl2.
//
// FIXME: this is a very quick impl. It must be improved.
func (c *cache) getTableMetaPair(ctx context.Context, tbl1, tbl2 source.Table) (md1, md2 *metadata.Table, err error) {
	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var mdErr error
		md1, mdErr = c.getTableMeta(gCtx, tbl1)
		return mdErr
	})
	g.Go(func() error {
		var mdErr error
		md2, mdErr = c.getTableMeta(gCtx, tbl2)
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
