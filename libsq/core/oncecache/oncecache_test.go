package oncecache_test

import (
	"context"
	"errors"
	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/oncecache"
	"github.com/neilotoole/sq/libsq/core/oncecache/example/hrsystem"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fetchEvenOnly(_ context.Context, key int) (string, error) {
	if key%2 == 0 {
		return strconv.Itoa(key), nil
	}
	return "", errors.New("odd numbers not supported")
}

func fetchDouble(_ context.Context, key int) (val int, err error) {
	return key * 2, nil
}

func TestCache(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := oncecache.New[int, string](fetchEvenOnly)

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
	c.Clear(ctx)
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

// TestContext verifies that the context passed to callbacks is decorated with
// the cache, as retrieved via [oncecache.FromContext].
func TestContext(t *testing.T) {
	ctx := context.Background()
	const cacheName = "test-cache"

	var c *oncecache.Cache[int, int]
	c = oncecache.New[int, int](
		func(ctx context.Context, key int) (val int, err error) {
			gotCache := oncecache.FromContext[int, int](ctx)
			require.Equal(t, c, gotCache)
			require.Equal(t, cacheName, gotCache.Name())

			val, err = fetchDouble(ctx, key)
			t.Logf("Fetch[%s](%v) (%v, %v)", c.Name(), key, val, err)
			return val, err
		},
		oncecache.Name(cacheName),
		oncecache.OnFill(func(ctx context.Context, key int, val int, err error) {
			gotCache := oncecache.FromContext[int, int](ctx)
			require.Equal(t, c, gotCache)
			require.Equal(t, cacheName, gotCache.Name())
			t.Logf("OnFill[%s](%v, %v, %v)", c.Name(), key, val, err)
		}),
		oncecache.OnEvict(func(ctx context.Context, key int, val int, err error) {
			gotCache := oncecache.FromContext[int, int](ctx)
			require.Equal(t, c, gotCache)
			require.Equal(t, cacheName, gotCache.Name())
			t.Logf("OnEvict[%s](%v, %v, %v)", c.Name(), key, val, err)
		}),
	)

	got, err := c.Get(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 2, got)

	c.Delete(ctx, 1)
}

const (
	acmeName    = "Acme Corporation"
	engDeptName = "Engineering"
	qaDeptName  = "QA"
	wileyName   = "Wile E. Coyote"
	wileyEmpID  = 1
)

func setupHRSystem(t *testing.T) (*hrsystem.HRDatabase, *hrsystem.HRCache) {
	t.Helper()
	log := slogt.New(t)

	db, err := hrsystem.NewHRDatabase(
		log.With("layer", "db"),
		"example/hrsystem/testdata/acme.json",
	)
	require.NoError(t, err)
	cache := hrsystem.NewHRCache(log.With("layer", "cache"), db)
	return db, cache
}

// Test_OnFill_OnEvict tests use of the [oncecache.OnFill] mechanism to
// propagate cache events between overlapping caches.
func Test_OnFill_OnEvict(t *testing.T) {
	ctx := context.Background()
	_, db := setupHRSystem(t)

	var (
		orgCache  *oncecache.Cache[string, *hrsystem.Org]
		deptCache *oncecache.Cache[string, *hrsystem.Department]
		empCache  *oncecache.Cache[int, *hrsystem.Employee]
	)

	orgCache = oncecache.New[string, *hrsystem.Org](
		db.GetOrg,
		oncecache.OnFill(func(ctx context.Context, orgName string, org *hrsystem.Org, err error) {
			// Propagate the org's departments to the deptCache.
			for _, dept := range org.Departments {
				deptCache.Set(ctx, dept.Name, dept, nil)
				// Note: Setting an entry on deptCache should in turn propagate to
				// empCache, because deptCache is itself configured with an OnFill
				// handler below.
			}
		}),
		oncecache.OnEvict(func(ctx context.Context, orgName string, org *hrsystem.Org, err error) {
			// As with OnFill, we'll propagate eviction.
			for _, dept := range org.Departments {
				deptCache.Delete(ctx, dept.Name)
			}
		}),
	)

	deptCache = oncecache.New[string, *hrsystem.Department](
		db.GetDepartment,
		oncecache.OnFill(func(ctx context.Context, deptName string, dept *hrsystem.Department, err error) {
			for _, emp := range dept.Staff {
				empCache.Set(ctx, emp.ID, emp, nil)
			}
		}),
		oncecache.OnEvict(func(ctx context.Context, deptName string, dept *hrsystem.Department, err error) {
			for _, emp := range dept.Staff {
				empCache.Delete(ctx, emp.ID)
			}
		}),
	)

	empCache = oncecache.New[int, *hrsystem.Employee](db.GetEmployee)

	// orgCache.Get should trigger entry propagation to the other caches.
	acmeCorp, err := orgCache.Get(ctx, acmeName)
	require.NoError(t, err)
	require.Equal(t, acmeName, acmeCorp.Name)
	require.Equal(t, 1, db.Stats().GetOrg())

	wiley, err := empCache.Get(ctx, wileyEmpID)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)
	require.Equal(t, 0, db.Stats().GetEmployee())

	engDept, err := deptCache.Get(ctx, engDeptName)
	require.NoError(t, err)
	require.Equal(t, engDeptName, engDept.Name)
	require.Equal(t, 0, db.Stats().GetDepartment())

	// Now we evict acmeCorp, which should propagate to the other caches.
	orgCache.Delete(ctx, acmeCorp.Name)

	// Wiley should no longer be cached, so this call should hit the db.
	wiley, err = empCache.Get(ctx, wileyEmpID)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)
	require.Equal(t, 1, db.Stats().GetEmployee())
}

// TestOnFillChan tests using the [oncecache.OnFillChan] mechanism to propagate
// cache entries between overlapping caches, using channels.
func TestOnFillChan(t *testing.T) {
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	_, db := setupHRSystem(t)

	var (
		orgCache  *oncecache.Cache[string, *hrsystem.Org]
		deptCache *oncecache.Cache[string, *hrsystem.Department]
		empCache  *oncecache.Cache[int, *hrsystem.Employee]
	)

	orgCacheCh := make(chan oncecache.Event[string, *hrsystem.Org], 10)
	defer close(orgCacheCh)

	orgCache = oncecache.New[string, *hrsystem.Org](
		db.GetOrg,
		oncecache.OnFillChan(orgCacheCh, false),
	)

	deptCacheCh := make(chan oncecache.Event[string, *hrsystem.Department], 10)
	defer close(deptCacheCh)

	deptCache = oncecache.New[string, *hrsystem.Department](
		db.GetDepartment,
		oncecache.OnFillChan(deptCacheCh, false),
	)

	empCache = oncecache.New[int, *hrsystem.Employee](
		func(ctx context.Context, key int) (val *hrsystem.Employee, err error) {
			t.Fatal("should not be called, because entries should have been propagated by now")
			return
		},
	)

	// We use handledCh to signal that an event has been handled.
	handledCh := make(chan struct{})
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-orgCacheCh:
				org := event.Val
				switch event.Action {
				case oncecache.ActionFill:
					for _, dept := range org.Departments {
						deptCache.Set(ctx, dept.Name, dept, event.Err)
					}
				case oncecache.ActionEvict:
					for _, dept := range org.Departments {
						deptCache.Delete(ctx, dept.Name)
					}
				default:
					t.Fatalf("unexpected action: %v", event.Action)
				}

			case event := <-deptCacheCh:
				dept := event.Val
				for _, emp := range dept.Staff {
					empCache.Set(ctx, emp.ID, emp, nil)
				}
			}
			handledCh <- struct{}{}
		}
	}()

	// orgCache.Get should trigger entry propagation to the other caches.
	acmeCorp, err := orgCache.Get(ctx, acmeName)
	require.NoError(t, err)
	require.Equal(t, acmeName, acmeCorp.Name)

	// Because we're using a goroutine for cache entry propagation, we wait for
	// two events to be handled: one for orgCache->deptCache, and another for
	// deptCache->empCache.

	<-handledCh
	<-handledCh

	wiley, err := empCache.Get(ctx, wileyEmpID)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)

	engDept, err := deptCache.Get(ctx, engDeptName)
	require.NoError(t, err)
	require.Equal(t, engDeptName, engDept.Name)
}

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
