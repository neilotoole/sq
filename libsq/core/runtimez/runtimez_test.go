package runtimez

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range 1000 {
				require.NotNil(t, MemStats())
			}
		}()
	}
	wg.Wait()
}

func TestStartMemStatsTracker(t *testing.T) {
	resetMemStatsState(t)
	MemStatsRefresh = time.Millisecond

	baseline := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	sys, curAllocs, totalAllocs, gcPauseNs := StartMemStatsTracker(ctx)
	require.NotNil(t, sys)
	require.NotNil(t, curAllocs)
	require.NotNil(t, totalAllocs)
	require.NotNil(t, gcPauseNs)

	// Allocate some garbage so the tracker observes non-zero values.
	sink := make([][]byte, 0, 128)
	for range 128 {
		sink = append(sink, make([]byte, 1<<16))
	}
	runtime.KeepAlive(sink)

	require.Eventually(t, func() bool {
		return sys.Load() > 0 && totalAllocs.Load() > 0
	}, 2*time.Second, time.Millisecond, "tracker should populate sys and totalAllocs")

	// Cancel and confirm the tracker goroutine exits (no leak). Its final
	// pass does an uncached ReadMemStats, so values must remain sensible. Poll
	// in this goroutine (not via require.Eventually, whose own poller goroutine
	// would skew the count) until the goroutine total returns to baseline.
	cancel()
	var exited bool
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if runtime.NumGoroutine() <= baseline {
			exited = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	require.True(t, exited, "tracker goroutine should exit when ctx is done")

	require.Positive(t, sys.Load())
	require.Positive(t, totalAllocs.Load())
}
