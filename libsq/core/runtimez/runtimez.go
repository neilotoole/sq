// Package runtimez provides functionality supplemental to stdlib's runtime pkg.
package runtimez

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// MemStatsRefresh determines how often the value returned by MemStats is
// refreshed.
var MemStatsRefresh = time.Millisecond * 250

var (
	memStats            *runtime.MemStats
	memStatsMu          sync.Mutex
	memStatsNextRefresh time.Time
)

// MemStats returns the most recent memory stats. To avoid slamming the runtime,
// stats are retrieved periodically per [MemStatsRefresh]. The caller must not
// mutate the returned [*runtime.MemStats], as the same instance is returned to
// other callers.
func MemStats() *runtime.MemStats {
	now := time.Now()
	memStatsMu.Lock()
	if now.After(memStatsNextRefresh) {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		memStats = &ms
		memStatsNextRefresh = now.Add(MemStatsRefresh)
	}
	memStatsMu.Unlock()

	return memStats
}

// StartMemStatsTracker starts a goroutine that tracks memory stats, returning
// the peak values of [runtime.MemStats.Sys], [runtime.MemStats.TotalAlloc] and
// [runtime.MemStats.PauseTotalNs]. The goroutine sleeps for sampleFreq between
// each sample and exits when ctx is done.
//
//nolint:revive // datarace
func StartMemStatsTracker(ctx context.Context) (sys, curAllocs, totalAllocs, gcPauseNs *atomic.Uint64) {
	lg.FromContext(ctx).Info("Starting memory stats tracker", lga.Freq, MemStatsRefresh)

	sys = &atomic.Uint64{}
	curAllocs = &atomic.Uint64{}
	totalAllocs = &atomic.Uint64{}
	gcPauseNs = &atomic.Uint64{}

	go func() {
		ticker := time.NewTicker(MemStatsRefresh)
		defer ticker.Stop()

		for {
			// Use our (cached) MemStats, which is updated periodically.
			stats := MemStats()
			if stats.Sys > sys.Load() {
				sys.Store(stats.Sys)
			}

			if stats.Alloc > curAllocs.Load() {
				curAllocs.Store(stats.Alloc)
			}

			totalAllocs.Store(stats.TotalAlloc)
			gcPauseNs.Store(stats.PauseTotalNs)

			select {
			case <-ctx.Done():
				// We're about to exit; this time we do a call to the (uncached)
				// runtime.ReadMemStats, just so we have fresh values on exit.
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				if ms.Sys > sys.Load() {
					sys.Store(stats.Sys)
				}

				totalAllocs.Store(ms.TotalAlloc)
				gcPauseNs.Store(ms.PauseTotalNs)
				return
			case <-ticker.C:
			}
		}
	}()

	return sys, curAllocs, totalAllocs, gcPauseNs
}
