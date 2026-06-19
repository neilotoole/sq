package secret_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/secret"
)

// countingResolver counts Resolve invocations.
type countingResolver struct {
	err   error
	value string
	count int
}

func (r *countingResolver) Resolve(_ context.Context, _ string) (string, error) {
	r.count++
	if r.err != nil {
		return "", r.err
	}
	return r.value, nil
}

// TestRegistry_MemoizesResolutions verifies that a Registry resolves each
// distinct scheme:path at most once for the Registry's lifetime (one CLI
// invocation). Several code paths resolve the same source location
// independently (e.g. --src.schema validation opens a probe connection,
// then Grips.doOpen opens the real one); without memoization each pass
// costs a fresh backend hit, which for the keyring scheme is an OS
// keychain roundtrip that may prompt the user.
func TestRegistry_MemoizesResolutions(t *testing.T) {
	ctx := context.Background()

	reg := secret.NewRegistry()
	counter := &countingResolver{value: "hunter2"}
	reg.Register("test", counter)

	for range 3 {
		got, err := reg.Expand(ctx, "postgres://alice:${test:pw}@db.acme.com/sakila")
		require.NoError(t, err)
		require.Contains(t, got, "hunter2")
	}
	require.Equal(t, 1, counter.count,
		"repeated resolution of the same placeholder should hit the backend once")

	// A distinct path is a distinct memo entry.
	_, err := reg.Expand(ctx, "${test:other}")
	require.NoError(t, err)
	require.Equal(t, 2, counter.count)

	// Failures are not memoized: each attempt retries the backend.
	failing := &countingResolver{err: errors.New("backend down")}
	reg.Register("fail", failing)
	for range 2 {
		_, err = reg.Expand(ctx, "${fail:x}")
		require.Error(t, err)
	}
	require.Equal(t, 2, failing.count,
		"failed resolutions must not be cached")
}

// slowResolver counts Resolve invocations and holds each one open long
// enough for concurrent callers to pile up.
type slowResolver struct {
	count atomic.Int32
}

func (r *slowResolver) Resolve(_ context.Context, _ string) (string, error) {
	r.count.Add(1)
	time.Sleep(50 * time.Millisecond)
	return "hunter2", nil
}

// TestRegistry_ConcurrentResolveSingleflight verifies that concurrent
// resolutions of the same scheme:path are coalesced into a single
// backend hit, mirroring the op resolver's singleflight behavior. The
// keyring backend does OS-keychain IPC that may prompt the user, so a
// fan-out of concurrent opens (e.g. parallel grip opens of sources
// sharing a placeholder) must never trigger duplicate prompts.
func TestRegistry_ConcurrentResolveSingleflight(t *testing.T) {
	ctx := context.Background()
	reg := secret.NewRegistry()
	resolver := &slowResolver{}
	reg.Register("test", resolver)

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	vals := make([]string, goroutines)
	for i := range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vals[i], errs[i] = reg.Expand(ctx, "${test:pw}")
		}()
	}
	wg.Wait()

	for i := range goroutines {
		require.NoError(t, errs[i])
		require.Equal(t, "hunter2", vals[i])
	}
	require.Equal(t, int32(1), resolver.count.Load(),
		"concurrent resolutions of one path must share a single backend hit")
}
