package footer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAlignRight_plain(t *testing.T) {
	t.Parallel()

	require.Equal(t, "    hi", alignRight("hi", 6))
	require.Equal(t, "hi", alignRight("hi", 0))
	require.Equal(t, "hello", alignRight("hello", 3))
}

func TestAlignRight_ansi(t *testing.T) {
	t.Parallel()

	line := "\x1b[1;93;48;5;226m\x1b[38;5;16mv0.54.0\x1b[0m"
	got := alignRight("Update available: "+line, 40)
	require.GreaterOrEqual(t, len(got), len("Update available: "))
	require.True(t, displayWidth(got) <= 40 || displayWidth("Update available: "+line) > 40)
}
