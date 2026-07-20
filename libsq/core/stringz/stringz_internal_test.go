package stringz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStringWithCharset tests the unexported stringWithCharset. Its
// empty-charset panic and non-positive-length branches are defensive and
// not reachable via the exported API, so they're exercised here directly.
func TestStringWithCharset(t *testing.T) {
	require.Equal(t, "", stringWithCharset(0, charsetAlphaLower))
	require.Equal(t, "", stringWithCharset(-1, charsetAlphaLower))

	got := stringWithCharset(10, charsetAlphaLower)
	require.Equal(t, 10, len(got))
	for _, r := range got {
		require.Contains(t, charsetAlphaLower, string(r))
	}

	require.Panics(t, func() {
		_ = stringWithCharset(1, "")
	})
}
