package oncecache_test

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/oncecache"
	"github.com/neilotoole/sq/libsq/core/oncecache/example/hrsystem"

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
	ctx := context.Background()
	c := oncecache.New[int, string](fetchEvenOnly)

	require.False(t, c.Has(0))

	got, err := c.Get(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, "0", got)
	require.True(t, c.Has(0))

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
		oncecache.OnFill(func(ctx context.Context, key, val int, err error) {
			gotCache := oncecache.FromContext[int, int](ctx)
			require.Equal(t, c, gotCache)
			require.Equal(t, cacheName, gotCache.Name())
			t.Logf("OnFill[%s](%v, %v, %v)", c.Name(), key, val, err)
		}),
		oncecache.OnEvict(func(ctx context.Context, key, val int, err error) {
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

func loadHRDatabase(t *testing.T) *hrsystem.HRDatabase {
	t.Helper()
	log := slogt.New(t)

	db, err := hrsystem.NewHRDatabase(
		log.With("layer", "db"),
		"example/hrsystem/testdata/acme.json",
	)
	require.NoError(t, err)
	return db
}

// Test_OnFill_OnEvict tests use of the [oncecache.OnFill] mechanism to
// propagate cache events between overlapping caches.
func Test_OnFill_OnEvict(t *testing.T) {
	var (
		ctx       = context.Background()
		db        = loadHRDatabase(t)
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

// Test_OnFillChan_OnEvictChan tests using the [oncecache.OnFillChan] mechanism
// to propagate cache entries between overlapping caches, using channels.
func Test_OnFillChan_OnEvictChan(t *testing.T) {
	log := slogt.New(t)
	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()

	db := loadHRDatabase(t)

	var (
		orgCache  *oncecache.Cache[string, *hrsystem.Org]
		deptCache *oncecache.Cache[string, *hrsystem.Department]
		empCache  *oncecache.Cache[int, *hrsystem.Employee]
	)

	orgCacheCh := make(chan oncecache.Event[string, *hrsystem.Org], 10)
	defer close(orgCacheCh)

	orgCache = oncecache.New[string, *hrsystem.Org](
		db.GetOrg,
		oncecache.Name("orgCache"),
		oncecache.OnFillChan(orgCacheCh, false),
		oncecache.OnEvictChan(orgCacheCh, false),
	)

	deptCacheCh := make(chan oncecache.Event[string, *hrsystem.Department], 10)
	defer close(deptCacheCh)

	deptCache = oncecache.New[string, *hrsystem.Department](
		db.GetDepartment,
		oncecache.Name("deptCache"),
		oncecache.OnFillChan(deptCacheCh, false),
		oncecache.OnEvictChan(deptCacheCh, false),
	)

	empCache = oncecache.New[int, *hrsystem.Employee](db.GetEmployee, oncecache.Name("empCache"))

	// We use actionCh to signal that an event has been handled.
	actionCh := make(chan oncecache.Action, 100)
	go func() {
		log := log.With("layer", "event")
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-orgCacheCh:
				log.Info("Got event", "e", event)
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
					panic(fmt.Sprintf("unexpected action: %v", event.Action))
				}
				actionCh <- event.Action
			case event := <-deptCacheCh:
				log.Info("Got event", "e", event)
				dept := event.Val
				switch event.Action {
				case oncecache.ActionFill:
					for _, emp := range dept.Staff {
						empCache.Set(ctx, emp.ID, emp, nil)
					}
				case oncecache.ActionEvict:
					for _, emp := range dept.Staff {
						empCache.Delete(ctx, emp.ID)
					}
				default:
					if event.Action.IsZero() {
						// This is the final zero event, indicating that the channel is closed.
						return
					}
					panic(fmt.Sprintf("unexpected action: %v", event.Action))
				}
				actionCh <- event.Action
			}
		}
	}()

	// orgCache.Get should trigger entry propagation to the other caches.
	acmeCorp, err := orgCache.Get(ctx, acmeName)
	require.NoError(t, err)
	require.Equal(t, acmeName, acmeCorp.Name)

	// Because we're using a goroutine for cache entry propagation, we need
	// to wait for 3 events to be handled:
	//
	// - fill orgCache[acmeName]
	// - fill deptCache[engDeptName]
	// - fill deptCache[qaDeptName]
	//
	// Note that other entry fills occur: in particular, empCache is populated
	// for each employee. However, this test hasn't set up a listener on empCache,
	// so empCache doesn't generate any events.
	requireDrainActionCh(t, actionCh, time.Millisecond, oncecache.ActionFill, 3)

	require.Equal(t, 0, db.Stats().GetEmployee())
	wiley, err := empCache.Get(ctx, wileyEmpID)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)
	require.Equal(t, 0, db.Stats().GetEmployee(), "shouldn't hit db")

	engDept, err := deptCache.Get(ctx, engDeptName)
	require.NoError(t, err)
	require.Equal(t, engDeptName, engDept.Name)
	require.Equal(t, 0, db.Stats().GetDepartment(), "shouldn't hit db")

	// Now we evict acmeCorp, which should propagate to the other caches.
	orgCache.Delete(ctx, acmeCorp.Name)
	// Similar to above, we should get three evictions.
	requireDrainActionCh(t, actionCh, time.Millisecond, oncecache.ActionEvict, 3)

	// Wiley should no longer be cached, so this call should hit the db.
	require.Equal(t, 0, db.Stats().GetEmployee())
	wiley, err = empCache.Get(ctx, wileyEmpID)
	require.NoError(t, err)
	require.Equal(t, wileyName, wiley.Name)
	require.Equal(t, 1, db.Stats().GetEmployee())
}

// requireDrainActionCh verifies that within timeout, ch receives exactly
// wantCount actions, all of which are wantAction.
func requireDrainActionCh(t *testing.T, ch <-chan oncecache.Action,
	timeout time.Duration, wantAction oncecache.Action, wantCount int,
) {
	t.Helper()

	ctx, cancel := context.WithCancelCause(context.Background())
	time.AfterFunc(timeout, func() {
		cancel(fmt.Errorf("timed out (%s) waiting for action", timeout))
	})

	var gotCount int
	var gotAction oncecache.Action
	for {
		select {
		case <-ctx.Done():
			if gotCount == wantCount {
				return
			}
			assert.Equal(t, wantCount, gotCount,
				"got %d actions in %s but wanted %d", gotCount, timeout, wantCount)
			require.NoError(t, context.Cause(ctx))
		case gotAction = <-ch:
		}

		if gotAction.IsZero() {
			break
		}

		gotCount++
		require.Equal(t, wantAction.String(), gotAction.String())
		require.LessOrEqual(t, gotCount, wantCount)
	}
	require.Equal(t, wantCount, gotCount)
}

func TestLogging(t *testing.T) {
	ctx := context.Background()

	c := oncecache.New[int, int](fetchDouble)

	gotName := c.Name()
	require.NotEmpty(t, gotName)
	t.Log(gotName)

	c = oncecache.New[int, int](fetchDouble, oncecache.Name("cache-foo"))
	gotName = c.Name()
	require.Equal(t, "cache-foo", gotName)

	// Sanity check: make sure Cache.LogValue doesn't shit the bed.
	log := slogt.New(t)
	log.Info("hello", "cache", c)

	s := c.String()
	require.Equal(t, "cache-foo[int, int][0]", s)
	_, _ = c.Get(ctx, 1)
	_, _ = c.Get(ctx, 2)
	_, _ = c.Get(ctx, 3)
	s = c.String()
	require.Equal(t, "cache-foo[int, int][3]", s)

	eventCh := make(chan oncecache.Event[int, int], 3)
	c = oncecache.New[int, int](
		fetchDouble,
		oncecache.Name("event-cache"),
		oncecache.OnFillChan(eventCh, false),
	)

	gotVal, gotErr := c.Get(ctx, 1)
	require.NoError(t, gotErr)
	require.Equal(t, 2, gotVal)

	time.Sleep(time.Millisecond) // Allow event to propagate
	var event oncecache.Event[int, int]
	select {
	case event = <-eventCh:
	default:
		t.Fatal("Expected event")
	}
	require.Equal(t, oncecache.ActionFill, event.Action)
	t.Logf("event: %s", event)
	t.Logf("entry: %s", event.Entry)

	log.Info("Got event", "event", event)
	log.Info("Got entry", "entry", event.Entry)
}
