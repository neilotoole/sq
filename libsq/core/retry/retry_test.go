package retry_test

import (
	"testing"
	"time"

	"github.com/neilotoole/sq/libsq/core/retry"
	"github.com/stretchr/testify/require"
)

func TestJitter(t *testing.T) {
	for i := 0; i < 1000; i++ {
		got := retry.Jitter()
		require.LessOrEqual(t, got, time.Millisecond*25)
		require.GreaterOrEqual(t, got, time.Millisecond*5)
	}
}
