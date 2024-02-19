package oncecache_test

import (
	"context"
	"github.com/neilotoole/sq/libsq/core/oncecache"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOnFill(t *testing.T) {
	ctx := context.Background()
	double := func(ctx context.Context, key int) (val int, err error) {
		return key * 2, nil
	}
	//c := oncecache.New[int, int](double)
	//
	//got, err := c.Get(ctx, 3)
	//require.NoError(t, err)
	//require.Equal(t, 6, got)

	//var cb oncecache.OnFillFunc[int, int] = func(ctx context.Context, key int, val int, err error) {
	//	require.NoError(t, err)
	//	require.Equal(t, 6, val)
	//}
	//_ = cb
	var cb oncecache.OnFillFunc[int, int] = func(ctx context.Context, key int, val int, err error) {
		t.Logf("key: %d, val: %d, err: %v", key, val, err)
	}

	//x := oncecache.OptHuzzah2[int, int]{Key: 1, Val: 2}

	c2 := oncecache.New[int, int](double, cb)
	_ = c2

	got, err := c2.Get(ctx, 3)
	require.NoError(t, err)
	require.Equal(t, 6, got)

}

func huzzah(ctx context.Context, key int, val int, err error) {
	println("huzzah")
}
