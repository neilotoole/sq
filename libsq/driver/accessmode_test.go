package driver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessMode_suffix(t *testing.T) {
	require.Equal(t, "rw", ModeReadWrite.suffix())
	require.Equal(t, "ro", ModeReadOnly.suffix())
	require.Equal(t, "rox", ModeReadOnlyExplicit.suffix())
	// suffixes must be distinct so the three modes get distinct cache keys.
	require.NotEqual(t, ModeReadOnly.suffix(), ModeReadOnlyExplicit.suffix())
}
