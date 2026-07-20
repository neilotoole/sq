package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoColorFor_FromEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	require.True(t, noColorFor(nil))
}

func TestNoColorFor_Default(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	require.False(t, noColorFor(nil))
}
