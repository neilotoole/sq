package runtimez

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

// resetMemStatsState restores the package-level mem-stats cache to its zero
// state and restores MemStatsRefresh when the test completes. Tests that touch
// the global cache must call this and must not run in parallel with each other.
func resetMemStatsState(t *testing.T) {
	t.Helper()
	prevRefresh := MemStatsRefresh
	memStatsMu.Lock()
	memStats = nil
	memStatsNextRefresh = time.Time{}
	memStatsMu.Unlock()
	t.Cleanup(func() {
		MemStatsRefresh = prevRefresh
		memStatsMu.Lock()
		memStats = nil
		memStatsNextRefresh = time.Time{}
		memStatsMu.Unlock()
	})
}

func TestMemStats_NonNil(t *testing.T) {
	resetMemStatsState(t)
	require.NotNil(t, MemStats())
}

func TestMemStats_Caches(t *testing.T) {
	resetMemStatsState(t)
	// A long refresh window means the second call must return the cached
	// instance, i.e. the identical pointer.
	MemStatsRefresh = time.Hour

	p1 := MemStats()
	require.NotNil(t, p1)
	p2 := MemStats()
	require.Same(t, p1, p2, "stats should be cached within the refresh window")
}

func TestMemStats_Refresh(t *testing.T) {
	resetMemStatsState(t)
	MemStatsRefresh = time.Hour

	p1 := MemStats()
	require.NotNil(t, p1)

	// Force the next-refresh deadline into the past; the next call must
	// re-read, yielding a fresh (different) instance.
	memStatsMu.Lock()
	memStatsNextRefresh = time.Now().Add(-time.Hour)
	memStatsMu.Unlock()

	p2 := MemStats()
	require.NotNil(t, p2)
	require.NotSame(t, p1, p2, "stats should be refreshed after the window elapses")
}

// TestMemStats_Concurrent hammers MemStats from many goroutines with a tiny
// refresh window so refreshes interleave with reads. Run under -race, this
// guards against an unsynchronized read of the shared memStats pointer.
func TestMemStats_Concurrent(t *testing.T) {
	resetMemStatsState(t)
	MemStatsRefresh = time.Nanosecond

	// Record a nil result rather than asserting inside the worker goroutines:
	// require.* calls t.FailNow (runtime.Goexit), which must run on the test
	// goroutine, so the workers flag failure and the test goroutine asserts.
	var sawNil atomic.Bool

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range 1000 {
				if MemStats() == nil {
					sawNil.Store(true)
					return
				}
			}
		}()
	}
	wg.Wait()
	require.False(t, sawNil.Load(), "MemStats must never return nil")
}

func TestStartMemStatsTracker(t *testing.T) {
	resetMemStatsState(t)
	MemStatsRefresh = time.Millisecond

	// Snapshot pre-existing goroutines so goleak only flags the tracker if it
	// fails to exit after cancel.
	ignoreExisting := goleak.IgnoreCurrent()

	ctx, cancel := context.WithCancel(context.Background())
	sys, curAllocs, totalAllocs, gcPauseNs := StartMemStatsTracker(ctx)
	require.NotNil(t, sys)
	require.NotNil(t, curAllocs)
	require.NotNil(t, totalAllocs)
	require.NotNil(t, gcPauseNs)

	// Allocate live heap so the tracker observes non-trivial Alloc/Sys, and
	// force a GC so PauseTotalNs becomes non-zero.
	sink := make([][]byte, 0, 128)
	for range 128 {
		sink = append(sink, make([]byte, 1<<16))
	}
	runtime.GC() //nolint:revive // explicit GC populates PauseTotalNs for the gcPauseNs assertion
	runtime.KeepAlive(sink)

	// All four peaks must be populated, including curAllocs and gcPauseNs,
	// which are the values the exit-branch refresh fix targets.
	require.Eventually(t, func() bool {
		return sys.Load() > 0 && curAllocs.Load() > 0 &&
			totalAllocs.Load() > 0 && gcPauseNs.Load() > 0
	}, 2*time.Second, time.Millisecond, "tracker should populate all four peak values")

	// Cancel and confirm the tracker goroutine exits (no leak). goleak retries
	// with backoff, so it absorbs the goroutine's shutdown latency.
	cancel()
	goleak.VerifyNone(t, ignoreExisting)

	require.Positive(t, sys.Load())
	require.Positive(t, curAllocs.Load())
	require.Positive(t, totalAllocs.Load())
	require.Positive(t, gcPauseNs.Load())
}
