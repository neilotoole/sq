package typetest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/testh/typetest"
)

func TestNotNullOf(t *testing.T) {
	m := typetest.NotNullOf[time.Time]()
	require.NoError(t, m.Match(time.Now()))
	require.ErrorContains(t, m.Match(nil), "want non-NULL time.Time")
	require.ErrorContains(t, m.Match("2017-07-07"), "got string")
	require.Equal(t, "NotNullOf[time.Time]", m.String())
	require.Equal(t, "NotNullOf[[]uint8]", typetest.NotNullOf[[]byte]().String())

	b := typetest.NotNullOf[[]byte]()
	require.ErrorContains(t, b.Match([]byte(nil)), "want non-NULL []uint8", "a nil slice is NULL")
	require.ErrorContains(t, b.Match([]byte(nil)), "[]byte(nil)", "the message tells a nil slice from an empty one")
	require.ErrorContains(t, m.Match(nil), "got NULL")
	require.NoError(t, b.Match([]byte{}), "an empty, non-nil slice is a value")
	require.NoError(t, b.Match([]byte("se")))
}
