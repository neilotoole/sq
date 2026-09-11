package debugz_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/debugz"
	"github.com/neilotoole/sq/libsq/core/options"
)

func TestOptProgressDebugSleep(t *testing.T) {
	opt := debugz.OptProgressDebugSleep
	require.Equal(t, "debug.progress.sleep", opt.Key())
	require.Equal(t, time.Duration(0), opt.Default())
	require.NotEmpty(t, opt.Usage())
	require.NotEmpty(t, opt.Help())

	// Default (nil options) is zero.
	require.Equal(t, time.Duration(0), opt.Get(nil))

	// Explicit value is returned.
	o := options.Options{opt.Key(): time.Second}
	require.Equal(t, time.Second, opt.Get(o))
}

func TestOptProgressDebugForce(t *testing.T) {
	opt := debugz.OptProgressDebugForce
	require.Equal(t, "debug.progress.force", opt.Key())
	require.False(t, opt.Default())
	require.NotEmpty(t, opt.Usage())
	require.NotEmpty(t, opt.Help())

	// Default (nil options) is false.
	require.False(t, opt.Get(nil))

	// Explicit value is returned.
	o := options.Options{opt.Key(): true}
	require.True(t, opt.Get(o))
}

// TestDebugSleep covers the DebugSleep branches. Its subtests measure short
// wall-clock intervals, so they deliberately don't run in parallel: scheduler
// jitter would make the timing assertions flaky.
//
//nolint:tparallel // Subtests are timing-sensitive; see the doc comment above.
func TestDebugSleep(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		sleep     time.Duration
		setOpt    bool
		wantSleep bool
	}{
		{name: "no_options", setOpt: false, wantSleep: false},
		{name: "zero", sleep: 0, setOpt: true, wantSleep: false},
		{name: "negative", sleep: -time.Second, setOpt: true, wantSleep: false},
		{name: "positive", sleep: 50 * time.Millisecond, setOpt: true, wantSleep: true},
	}

	for _, tc := range testCases {
		// Note: these subtests deliberately don't call t.Parallel(). They
		// measure short wall-clock intervals, and parallel scheduler delays
		// can push a no-sleep case past its upper bound, causing flakiness.
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.setOpt {
				ctx = options.NewContext(ctx,
					options.Options{debugz.OptProgressDebugSleep.Key(): tc.sleep})
			}

			start := time.Now()
			debugz.DebugSleep(ctx)
			elapsed := time.Since(start)

			if tc.wantSleep {
				require.GreaterOrEqual(t, elapsed, tc.sleep)
			} else {
				// Generous upper bound: we only need to prove DebugSleep
				// didn't block for the configured duration, without being
				// sensitive to scheduler jitter under -race or loaded CI.
				require.Less(t, elapsed, 5*time.Second)
			}
		})
	}
}

// TestDebugSleep_canceled verifies that a canceled context interrupts the
// sleep rather than blocking for the full duration.
func TestDebugSleep_canceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = options.NewContext(ctx,
		options.Options{debugz.OptProgressDebugSleep.Key(): 30 * time.Second})

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	debugz.DebugSleep(ctx)
	require.Less(t, time.Since(start), 5*time.Second,
		"DebugSleep should return promptly when ctx is canceled")
}
