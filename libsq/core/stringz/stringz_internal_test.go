package stringz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStringWithCharset tests the unexported stringWithCharset. Its
// empty-charset panic and non-positive-length branches are defensive and
// not reachable via the exported API, so they're exercised here directly.
func TestStringWithCharset(t *testing.T) {
	require.Equal(t, "", stringWithCharset(0, CharsetAlphaLower))
	require.Equal(t, "", stringWithCharset(-1, CharsetAlphaLower))

	got := stringWithCharset(10, CharsetAlphaLower)
	require.Equal(t, 10, len(got))
	for _, r := range got {
		require.Contains(t, CharsetAlphaLower, string(r))
	}

	require.Panics(t, func() {
		_ = stringWithCharset(1, "")
	})
}
