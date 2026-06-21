package retry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFibonacci verifies that newFibonacci caps the backoff interval at
// maxFibBackoff (5s), while leaving smaller intervals untouched.
func TestFibonacci(t *testing.T) {
	// Base of 1s so the sequence (1s, 2s, 3s, 5s, 8s, ...) crosses the
	// 5s cap quickly, exercising both the capped and uncapped branches.
	b := newFibonacci(time.Second)

	var (
		sawUncapped bool
		maxSeen     time.Duration
	)
	for range 10 {
		d, stop := b.Next()
		// A bare fibonacci backoff never signals stop on its own.
		require.False(t, stop)
		require.LessOrEqual(t, d, maxFibBackoff, "interval must never exceed the cap")
		if d < maxFibBackoff {
			sawUncapped = true
		}
		if d > maxSeen {
			maxSeen = d
		}
	}

	require.True(t, sawUncapped, "early intervals should be below the cap")
	require.Equal(t, maxFibBackoff, maxSeen, "later intervals should be clamped to the cap")
}
