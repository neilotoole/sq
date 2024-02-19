package diff

import (
	"context"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
	"sync"
)

type cache struct {
	mu      sync.Mutex
	ru      *run.Run
	tblMeta map[source.Table]*metadata.Table
}

// getTableMeta returns the [metadata.Table] for tbl. If the table does not
// exist, then (nil,nil) is returned, not an error.
//
// The caller must not mutate the returned [metadata.Table]. Use
// [metadata.Table.Clone] if necessary.
func (c *cache) getTableMeta(ctx context.Context, tbl source.Table) (*metadata.Table, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	md, ok := c.tblMeta[tbl]
	if ok {
		return md, nil
	}

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
