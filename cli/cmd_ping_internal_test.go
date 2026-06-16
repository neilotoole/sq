package cli

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// fakePingProvider returns drvr for any source type.
type fakePingProvider struct{ drvr driver.Driver }

func (p fakePingProvider) DriverFor(drivertype.Type) (driver.Driver, error) {
	return p.drvr, nil
}

// fakePingDriver implements driver.Driver but overrides only Ping;
// pingSource calls nothing else on the driver. Other methods panic via
// the embedded nil interface if ever reached.
type fakePingDriver struct {
	driver.Driver
	pingFn func(ctx context.Context, src *source.Source, mode driver.AccessMode) error
}

func (d *fakePingDriver) Ping(ctx context.Context, src *source.Source, mode driver.AccessMode) error {
	return d.pingFn(ctx, src, mode)
}

// TestPingSource_TimeoutDoesNotLeakGoroutine pins the buffered-doneCh
// fix: when a ping times out, pingSource returns via ctx.Done() with no
// receiver left for doneCh, so the ping goroutine must still be able to
// complete its send (doneCh is buffered, size 1) and exit. An unbuffered
// doneCh would block that send forever, leaking the goroutine.
func TestPingSource_TimeoutDoesNotLeakGoroutine(t *testing.T) {
	// Not parallel: this test counts goroutines, so it must not race
	// other tests' goroutine churn.
	release := make(chan struct{})
	var once sync.Once
	releaseFn := func() { once.Do(func() { close(release) }) }
	t.Cleanup(releaseFn) // Unblock Ping even if an assertion fails early.

	drvr := &fakePingDriver{
		pingFn: func(_ context.Context, _ *source.Source, _ driver.AccessMode) error {
			<-release // Block well past the ping timeout.
			return nil
		},
	}
	dp := fakePingProvider{drvr: drvr}
	src := &source.Source{
		Handle:   "@p",
		Type:     drivertype.SQLite,
		Location: "sqlite3:///tmp/sq_ping_leak_test.db", // No placeholders: ResolveSourceSecrets is a no-op.
	}
	resultCh := make(chan pingResult, 1)

	n0 := runtime.NumGoroutine()
	pingSource(context.Background(), dp, src, 20*time.Millisecond, resultCh)

	res := <-resultCh
	require.ErrorIs(t, res.err, context.DeadlineExceeded,
		"timeout path must return a deadline error")

	// The ping goroutine is still blocked in Ping. Release it; with a
	// buffered doneCh its send completes and it exits. Poll with a bare
	// sleep loop rather than require.Eventually, which spawns its own
	// goroutines and would inflate the count we're measuring.
	releaseFn()
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > n0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	require.LessOrEqual(t, runtime.NumGoroutine(), n0,
		"ping goroutine must exit after Ping returns; if it doesn't, doneCh blocked the send (leak)")
}
