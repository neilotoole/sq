package oncecache_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/neilotoole/sq/libsq/core/oncecache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func noOddNumbers(_ context.Context, key int) (string, error) {
	if key%2 == 0 {
		return strconv.Itoa(key), nil
	}
	return "", errors.New("odd numbers not supported")
}

func TestCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	fetcher := noOddNumbers
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
		return noOddNumbers(ctx, key)
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
