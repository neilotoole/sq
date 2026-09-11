package mdcache_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/mdcache"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCache_Close_nil(t *testing.T) {
	var c *mdcache.Cache
	err := c.Close()
	require.NoError(t, err)
}

func TestCache_New(t *testing.T) {
	coll := &source.Collection{}
	// Note: grips is nil here, which is fine for construction.
	// Actual operations would fail, but we're just testing construction.
	c := mdcache.New(nil, coll, nil)
	require.NotNil(t, c)

	err := c.Close()
	require.NoError(t, err)
}

func TestCache_SourceMetaPair_nilSources(t *testing.T) {
	coll := &source.Collection{}
	c := mdcache.New(nil, coll, nil)
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()

	// Both nil
	md1, md2, err := c.SourceMetaPair(ctx, nil, nil)
	require.Error(t, err)
	require.Nil(t, md1)
	require.Nil(t, md2)
	require.Contains(t, err.Error(), "must not be nil")

	// First nil
	src := &source.Source{Handle: "@test"}
	md1, md2, err = c.SourceMetaPair(ctx, nil, src)
	require.Error(t, err)
	require.Nil(t, md1)
	require.Nil(t, md2)

	// Second nil
	md1, md2, err = c.SourceMetaPair(ctx, src, nil)
	require.Error(t, err)
	require.Nil(t, md1)
	require.Nil(t, md2)
}

func TestCache_TableNamesPair_nilSources(t *testing.T) {
	coll := &source.Collection{}
	c := mdcache.New(nil, coll, nil)
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()

	// Both nil
	tbls1, tbls2, err := c.TableNamesPair(ctx, nil, nil)
	require.Error(t, err)
	require.Nil(t, tbls1)
	require.Nil(t, tbls2)
	require.Contains(t, err.Error(), "must not be nil")

	// First nil
	src := &source.Source{Handle: "@test"}
	tbls1, tbls2, err = c.TableNamesPair(ctx, nil, src)
	require.Error(t, err)
	require.Nil(t, tbls1)
	require.Nil(t, tbls2)

	// Second nil
	tbls1, tbls2, err = c.TableNamesPair(ctx, src, nil)
	require.Error(t, err)
	require.Nil(t, tbls1)
	require.Nil(t, tbls2)
}

func TestCache_DBPropertiesPair_nilSources(t *testing.T) {
	coll := &source.Collection{}
	c := mdcache.New(nil, coll, nil)
	t.Cleanup(func() { _ = c.Close() })

	ctx := context.Background()

	// Both nil
	dbp1, dbp2, err := c.DBPropertiesPair(ctx, nil, nil)
	require.Error(t, err)
	require.Nil(t, dbp1)
	require.Nil(t, dbp2)
	require.Contains(t, err.Error(), "must not be nil")

	// First nil
	src := &source.Source{Handle: "@test"}
	dbp1, dbp2, err = c.DBPropertiesPair(ctx, nil, src)
	require.Error(t, err)
	require.Nil(t, dbp1)
	require.Nil(t, dbp2)

	// Second nil
	dbp1, dbp2, err = c.DBPropertiesPair(ctx, src, nil)
	require.Error(t, err)
	require.Nil(t, dbp1)
	require.Nil(t, dbp2)
}

// newCacheWithSL3 returns a Cache backed by the embedded SQLite sakila
// source (@sakila_sl3), which is available under -short. The returned
// cache is closed via t.Cleanup.
func newCacheWithSL3(t *testing.T) (*mdcache.Cache, *source.Source) {
	t.Helper()
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	coll := th.NewCollection(sakila.SL3)
	c := mdcache.New(th.Log(), coll, th.Grips())
	t.Cleanup(func() { _ = c.Close() })
	return c, src
}

func TestCache_SourceMeta(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	md, err := c.SourceMeta(ctx, src.Handle)
	require.NoError(t, err)
	require.NotNil(t, md)
	require.Equal(t, src.Handle, md.Handle)

	// Second call should hit the cache and return the same pointer.
	md2, err := c.SourceMeta(ctx, src.Handle)
	require.NoError(t, err)
	require.Same(t, md, md2)

	// Unknown handle should error (collection lookup fails).
	_, err = c.SourceMeta(ctx, "@does_not_exist")
	require.Error(t, err)
}

func TestCache_TableMeta(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	tbl := source.Table{Handle: src.Handle, Name: sakila.TblActor}
	md, err := c.TableMeta(ctx, tbl)
	require.NoError(t, err)
	require.NotNil(t, md)
	require.Equal(t, sakila.TblActor, md.Name)

	// Cached call returns the same pointer.
	md2, err := c.TableMeta(ctx, tbl)
	require.NoError(t, err)
	require.Same(t, md, md2)

	// A nonexistent table triggers the NotExistError branch, which
	// returns (nil, nil).
	missing := source.Table{Handle: src.Handle, Name: "no_such_table_xyz"}
	md3, err := c.TableMeta(ctx, missing)
	require.NoError(t, err)
	require.Nil(t, md3)

	// Unknown handle should error (collection lookup fails).
	_, err = c.TableMeta(ctx, source.Table{Handle: "@nope", Name: sakila.TblActor})
	require.Error(t, err)
}

func TestCache_TableNames(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	tbls, err := c.TableNames(ctx, src.Handle)
	require.NoError(t, err)
	require.Contains(t, tbls, sakila.TblActor)
	require.Contains(t, tbls, sakila.TblFilm)

	// Unknown handle should error.
	_, err = c.TableNames(ctx, "@does_not_exist")
	require.Error(t, err)
}

func TestCache_DBProperties(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	props, err := c.DBProperties(ctx, src.Handle)
	require.NoError(t, err)
	require.NotEmpty(t, props)

	// Unknown handle should error.
	_, err = c.DBProperties(ctx, "@does_not_exist")
	require.Error(t, err)
}

func TestCache_TableMetaPair(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	tbl1 := source.Table{Handle: src.Handle, Name: sakila.TblActor}
	tbl2 := source.Table{Handle: src.Handle, Name: sakila.TblFilm}

	// Neither cached yet: parallel fetch path in getPair.
	md1, md2, err := c.TableMetaPair(ctx, tbl1, tbl2)
	require.NoError(t, err)
	require.NotNil(t, md1)
	require.NotNil(t, md2)
	require.Equal(t, sakila.TblActor, md1.Name)
	require.Equal(t, sakila.TblFilm, md2.Name)

	// Both cached now: sequential path in getPair (has1 && has2).
	md1b, md2b, err := c.TableMetaPair(ctx, tbl1, tbl2)
	require.NoError(t, err)
	require.Same(t, md1, md1b)
	require.Same(t, md2, md2b)
}

func TestCache_TableMetaPair_oneCached(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	tbl1 := source.Table{Handle: src.Handle, Name: sakila.TblActor}
	tbl2 := source.Table{Handle: src.Handle, Name: sakila.TblFilm}

	// Prime only tbl1.
	_, err := c.TableMeta(ctx, tbl1)
	require.NoError(t, err)

	// has1 && !has2: takes the sequential branch (has1 || has2).
	md1, md2, err := c.TableMetaPair(ctx, tbl1, tbl2)
	require.NoError(t, err)
	require.NotNil(t, md1)
	require.NotNil(t, md2)
	require.Equal(t, sakila.TblActor, md1.Name)
	require.Equal(t, sakila.TblFilm, md2.Name)
}

func TestCache_TableMetaPair_parallelError(t *testing.T) {
	c, _ := newCacheWithSL3(t)
	ctx := context.Background()

	// Neither key cached, both reference unknown handles: parallel
	// fetch path, both legs error, g.Wait returns an error.
	tbl1 := source.Table{Handle: "@nope1", Name: "t1"}
	tbl2 := source.Table{Handle: "@nope2", Name: "t2"}
	_, _, err := c.TableMetaPair(ctx, tbl1, tbl2)
	require.Error(t, err)
}

func TestCache_SourceMetaPair(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	md1, md2, err := c.SourceMetaPair(ctx, src, src)
	require.NoError(t, err)
	require.NotNil(t, md1)
	require.NotNil(t, md2)
	// Same handle: both legs resolve to the same cached entry.
	require.Same(t, md1, md2)
}

func TestCache_TableNamesPair(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	tbls1, tbls2, err := c.TableNamesPair(ctx, src, src)
	require.NoError(t, err)
	require.Contains(t, tbls1, sakila.TblActor)
	require.Contains(t, tbls2, sakila.TblActor)
}

func TestCache_DBPropertiesPair(t *testing.T) {
	c, src := newCacheWithSL3(t)
	ctx := context.Background()

	dbp1, dbp2, err := c.DBPropertiesPair(ctx, src, src)
	require.NoError(t, err)
	require.NotEmpty(t, dbp1)
	require.NotEmpty(t, dbp2)
}

// newCacheWithBadSrc returns a Cache whose collection holds a single
// source whose grip cannot be opened (its location lacks the required
// "sqlite3://" prefix). Every fetch therefore fails at the db()/grip
// resolution step, exercising those error branches.
func newCacheWithBadSrc(t *testing.T) *mdcache.Cache {
	t.Helper()
	th := testh.New(t)
	// Trigger internal initialization of the registry/grips.
	_ = th.Source(sakila.SL3)
	coll := &source.Collection{}
	bad := &source.Source{
		Handle:   "@bad",
		Type:     drivertype.SQLite,
		Location: "sqlite3:/missing-prefix/foo.db",
	}
	require.NoError(t, coll.Add(bad))
	c := mdcache.New(th.Log(), coll, th.Grips())
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestCache_fetch_openError(t *testing.T) {
	c := newCacheWithBadSrc(t)
	ctx := context.Background()

	_, err := c.DBProperties(ctx, "@bad")
	require.Error(t, err)

	_, err = c.TableNames(ctx, "@bad")
	require.Error(t, err)

	_, err = c.SourceMeta(ctx, "@bad")
	require.Error(t, err)

	_, err = c.TableMeta(ctx, source.Table{Handle: "@bad", Name: sakila.TblActor})
	require.Error(t, err)
}

// newCacheWithCorruptSrc returns a Cache whose collection holds a single
// source that points at an on-disk file which is not a valid SQLite
// database. The grip opens, but driver queries fail, exercising the
// non-NotExist error branches of the fetch functions.
func newCacheWithCorruptSrc(t *testing.T) *mdcache.Cache {
	t.Helper()
	th := testh.New(t)
	_ = th.Source(sakila.SL3)

	p := filepath.Join(t.TempDir(), "corrupt.db")
	require.NoError(t, os.WriteFile(p,
		[]byte("this is not a sqlite database file, just junk bytes here"), 0o600))

	coll := &source.Collection{}
	bad := &source.Source{
		Handle:   "@corrupt",
		Type:     drivertype.SQLite,
		Location: "sqlite3://" + p,
	}
	require.NoError(t, coll.Add(bad))
	c := mdcache.New(th.Log(), coll, th.Grips())
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestCache_fetch_queryError(t *testing.T) {
	c := newCacheWithCorruptSrc(t)
	ctx := context.Background()

	_, err := c.DBProperties(ctx, "@corrupt")
	require.Error(t, err)

	_, err = c.TableNames(ctx, "@corrupt")
	require.Error(t, err)

	_, err = c.SourceMeta(ctx, "@corrupt")
	require.Error(t, err)

	_, err = c.TableMeta(ctx, source.Table{Handle: "@corrupt", Name: sakila.TblActor})
	require.Error(t, err)
}

// TestCache_fetch_driverError exercises the error branches inside the
// fetch functions that occur after db()/grip resolution succeeds but the
// driver metadata call itself fails. It primes the grip with a successful
// call (so the grip is cached and db() returns the already-open handle),
// then issues fetches with a canceled context: db() still succeeds, but
// the driver query fails with a non-NotExist error.
func TestCache_fetch_driverError(t *testing.T) {
	th := testh.New(t)
	src := th.Source(sakila.SL3)
	coll := th.NewCollection(sakila.SL3)

	// Prime the grip via one cache so the underlying *sql.DB is open
	// and cached in th.Grips().
	primer := mdcache.New(th.Log(), coll, th.Grips())
	t.Cleanup(func() { _ = primer.Close() })
	_, err := primer.DBProperties(context.Background(), src.Handle)
	require.NoError(t, err)

	// Use a fresh cache (empty) over the same grips, with a canceled
	// context. db() resolves the cached grip without re-pinging, then
	// the driver query fails with "context canceled".
	c := mdcache.New(th.Log(), coll, th.Grips())
	t.Cleanup(func() { _ = c.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = c.DBProperties(ctx, src.Handle)
	require.Error(t, err)

	_, err = c.TableNames(ctx, src.Handle)
	require.Error(t, err)

	_, err = c.SourceMeta(ctx, src.Handle)
	require.Error(t, err)

	_, err = c.TableMeta(ctx, source.Table{Handle: src.Handle, Name: sakila.TblActor})
	require.Error(t, err)
}
