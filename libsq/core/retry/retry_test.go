package retry_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/retry"
)

func TestJitter(t *testing.T) {
	for range 1000 {
		got := retry.Jitter()
		require.LessOrEqual(t, got, time.Millisecond*25)
		require.GreaterOrEqual(t, got, time.Millisecond*5)
	}
}

func TestSleepJitter(t *testing.T) {
	start := time.Now()
	retry.SleepJitter()
	elapsed := time.Since(start)
	// SleepJitter sleeps for Jitter(), which is at least 5ms.
	require.GreaterOrEqual(t, elapsed, time.Millisecond*5)
}

func TestMatch(t *testing.T) {
	fn := retry.Match("refused")
	require.NotNil(t, fn)

	require.False(t, fn(nil), "nil error must never match")
	require.True(t, fn(errors.New("connection refused")), "substring present")
	require.False(t, fn(errors.New("some other problem")), "substring absent")

	// Empty match string is contained in every non-nil error.
	empty := retry.Match("")
	require.True(t, empty(errors.New("anything")))
	require.False(t, empty(nil))
}

func TestDo_FirstTrySuccess(t *testing.T) {
	var calls int
	err := retry.Do(context.Background(), time.Second, func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls, "fn that succeeds must be called exactly once")
}

func TestDo_EventualSuccess(t *testing.T) {
	// Exercises the fibonacci backoff path (newFibonacci) with a real retry.
	var calls int
	err := retry.Do(context.Background(), time.Second*10, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 3, calls)
}

func TestDo_NoMatchReturnsImmediately(t *testing.T) {
	sentinel := errors.New("fatal: not retryable")
	var calls int
	err := retry.Do(context.Background(), time.Second*10, func() error {
		calls++
		return sentinel
	}, retry.Match("some-substring-that-does-not-appear"))

	require.Error(t, err)
	require.ErrorIs(t, err, sentinel, "the unmatched error is returned verbatim")
	require.Equal(t, 1, calls, "unmatched error must not be retried")
}

func TestDo_MatchRetriesUntilMaxDuration(t *testing.T) {
	var calls int
	err := retry.DoConstant(context.Background(), time.Millisecond, time.Millisecond*50, func() error {
		calls++
		return errors.New("connection refused")
	}, retry.Match("refused"))

	require.Error(t, err)
	require.Greater(t, calls, 1, "matched error must be retried")
}

func TestDo_MultipleMatchFuncs(t *testing.T) {
	// Only the second MatchFunc matches; the loop must still find it.
	var calls int
	err := retry.DoConstant(context.Background(), time.Millisecond, time.Millisecond*50, func() error {
		calls++
		return errors.New("bar happened")
	}, retry.Match("foo"), retry.Match("bar"))

	require.Error(t, err)
	require.Greater(t, calls, 1, "error matched by any MatchFunc must be retried")
}

func TestDo_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var calls int
	err := retry.Do(ctx, 0, func() error {
		calls++
		return errors.New("x")
	})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, calls, "fn must not be invoked once ctx is already canceled")
}

func TestDo_MaxDurationZero_StopsOnContext(t *testing.T) {
	// maxDuration == 0 means "no max duration"; ctx is the only bound. Cancel
	// from within fn after a fixed number of attempts so the call count is
	// deterministic rather than dependent on backoff vs. deadline timing.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int
	err := retry.Do(ctx, 0, func() error {
		calls++
		if calls == 3 {
			cancel()
		}
		return errors.New("persistent")
	})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 3, calls, "loop must stop once ctx is canceled, with no further attempt")
}

func TestDoConstant_FirstTrySuccess(t *testing.T) {
	var calls int
	err := retry.DoConstant(context.Background(), time.Millisecond, time.Second, func() error {
		calls++
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestDoConstant_NonPositiveIntervalPanics(t *testing.T) {
	// The underlying goretry.NewConstant panics on a non-positive interval.
	for _, interval := range []time.Duration{0, -time.Millisecond} {
		require.Panics(t, func() {
			_ = retry.DoConstant(context.Background(), interval, time.Second, func() error {
				return nil
			})
		})
	}
}

func TestDoConstant_EventualSuccess(t *testing.T) {
	var calls int
	err := retry.DoConstant(context.Background(), time.Millisecond, time.Second, func() error {
		calls++
		if calls < 5 {
			return errors.New("transient")
		}
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, 5, calls)
}

func TestDoConstant_MaxDurationReached(t *testing.T) {
	// This is the one deliberately timing-based test: it verifies that
	// maxDuration terminates the loop. A 1ms interval against a 200ms cap
	// leaves a wide enough margin that "retried, then stopped, bounded" holds
	// even under a loaded scheduler.
	start := time.Now()
	var calls int
	err := retry.DoConstant(context.Background(), time.Millisecond, time.Millisecond*200, func() error {
		calls++
		return errors.New("persistent")
	})
	require.Error(t, err)
	require.Greater(t, calls, 1)
	require.Less(t, time.Since(start), time.Second*5, "must stop near maxDuration, not run forever")
}

func TestDoConstant_MaxDurationZero_StopsOnContext(t *testing.T) {
	// maxDuration == 0 means "no max duration"; ctx is the only bound. Cancel
	// from within fn after a fixed number of attempts for a deterministic count.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var calls int
	err := retry.DoConstant(ctx, time.Millisecond, 0, func() error {
		calls++
		if calls == 3 {
			cancel()
		}
		return errors.New("persistent")
	})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 3, calls, "loop must stop once ctx is canceled, with no further attempt")
}
