package typetest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/typetest"
)

func TestSpacePadded(t *testing.T) {
	require.Equal(t, "ab   ", typetest.SpacePadded("ab", 5))
	require.Equal(t, "     ", typetest.SpacePadded("", 5))
	require.Equal(t, "abc", typetest.SpacePadded("abc", 3))
}

func TestZeroPadded(t *testing.T) {
	require.Equal(t, []byte{'a', 0, 0}, typetest.ZeroPadded([]byte("a"), 3))
	require.Equal(t, []byte{0, 0}, typetest.ZeroPadded(nil, 2))
}

func TestTimeOfDay(t *testing.T) {
	require.Equal(t, time.Date(1, time.January, 1, 12, 34, 56, 789000000, time.UTC),
		typetest.TimeOfDay(12, 34, 56, 789000000))
}

func TestCase_Creates(t *testing.T) {
	c := intCase("x")
	require.True(t, c.Creates(typetest.NotNull))
	require.True(t, c.Creates(typetest.Nullable))

	c = c.WithVariants(typetest.NotNull)
	require.True(t, c.Creates(typetest.NotNull))
	require.False(t, c.Creates(typetest.Nullable))
}
