package oncecache_test

import (
	"context"
	"errors"
	"github.com/neilotoole/slogt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/neilotoole/sq/libsq/core/oncecache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fetchEvenOnly(_ context.Context, key int) (string, error) {
	if key%2 == 0 {
		return strconv.Itoa(key), nil
	}
	return "", errors.New("odd numbers not supported")
}

func TestCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fetcher := fetchEvenOnly
	c := oncecache.New[int, string](fetcher)

	got, err := c.Get(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "0", got)

	got, err = c.Get(ctx, 1)
	require.Error(t, err)
	require.Empty(t, got)

	// Seven is my lucky number though.
	c.Set(ctx, 7, "seven", nil)
	got, err = c.Get(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, "seven", got)

	// Verify that it a value can only be set once.
	c.Set(ctx, 7, "", errors.New("nope"))
	got, err = c.Get(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, "seven", got)

	// But, if we delete the entry, it can be set again.
	c.Delete(ctx, 7)
	got, err = c.Get(ctx, 7)
	require.Error(t, err)
	require.Empty(t, got)

	// Verify that clear works too.
	c.Clear(nil)
	c.Set(ctx, 7, "seven", nil)
	got, err = c.Get(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, "seven", got)
}

func TestCacheConcurrent(t *testing.T) {
	t.Parallel()

	const concurrency = 1000
	const numbers = 500
	ctx := context.Background()

	// invocations tracks how many times fetcher is invoked for each key.
	// Hint: it should be invoked only once per key.
	invocations := map[int]*atomic.Int64{}
	for i := 0; i < numbers; i++ {
		invocations[i] = &atomic.Int64{}
	}

	fetcher := func(ctx context.Context, key int) (val string, err error) {
		invocations[key].Add(1)
		return fetchEvenOnly(ctx, key)
	}

	c := oncecache.New[int, string](fetcher)

	wg := &sync.WaitGroup{}
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numbers; j++ {
				got, err := c.Get(ctx, j)
				if j%2 == 0 {
					require.NoError(t, err)
					require.Equal(t, strconv.Itoa(j), got)
				} else {
					require.Error(t, err)
					require.Empty(t, got)
				}
			}
		}()
	}
	wg.Wait()

	for i := 0; i < numbers; i++ {
		assert.Equal(t, int64(1), invocations[i].Load(), "key %d", i)
	}
}

//	func fetchDouble[K ~int, V ~int](_ context.Context, key K) (val V, err error) {
//		return V(key * 2), nil
//	}
func fetchDouble(_ context.Context, key int) (val int, err error) {
	return key * 2, nil
}

//func TestOnFillChan(t *testing.T) {
//	ctx := context.Background()
//
//	var opt oncecache.Opt[int, int]
//
//	eventCh := make(chan oncecache.Event[int, int], 10)
//
//	c2 := oncecache.New[int, int](fetchDouble[int, int], opt)
//
//	got, err := c2.Get(ctx, 3)
//	require.NoError(t, err)
//	require.Equal(t, 6, got)
//
//}

func TestLogging(t *testing.T) {
	ctx := context.Background()

	c := oncecache.New[int, int](fetchDouble)
	got := c.Name()
	require.NotEmpty(t, got)
	t.Log(got)

	c = oncecache.New[int, int](fetchDouble, oncecache.Name("cache-foo"))
	got = c.Name()
	require.Equal(t, "cache-foo", got)

	// Sanity check: make sure Cache.LogValue doesn't shit the bed.
	slogt.New(t).Info("hello", "cache", c)

	s := c.String()
	require.Equal(t, "cache-foo[int, int][0]", s)
	_, _ = c.Get(ctx, 1)
	_, _ = c.Get(ctx, 2)
	_, _ = c.Get(ctx, 3)
	s = c.String()
	require.Equal(t, "cache-foo[int, int][3]", s)
}

//func TestOnFill(t *testing.T) {
//	ctx := context.Background()
//	double := func(ctx context.Context, key int) (val int, err error) {
//		return key * 2, nil
//	}
//	//c := oncecache.New[int, int](double)
//	//
//	//got, err := c.Get(ctx, 3)
//	//require.NoError(t, err)
//	//require.Equal(t, 6, got)
//
//	//var cb oncecache.OnFillFunc[int, int] = func(ctx context.Context, key int, val int, err error) {
//	//	require.NoError(t, err)
//	//	require.Equal(t, 6, val)
//	//}
//	//_ = cb
//	var cb oncecache.OnFillFunc[int, int] = func(ctx context.Context, key int, val int, err error) {
//		t.Logf("key: %d, val: %d, err: %v", key, val, err)
//	}
//
//	//x := oncecache.OptHuzzah2[int, int]{Key: 1, Val: 2}
//
//	c2 := oncecache.New[int, int](double, cb)
//	_ = c2
//
//	got, err := c2.Get(ctx, 3)
//	require.NoError(t, err)
//	require.Equal(t, 6, got)
//
//}
