package mdcache_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/mdcache"
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
