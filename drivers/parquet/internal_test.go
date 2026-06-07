package parquet

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrw(t *testing.T) {
	// errw(nil) returns nil; errz.Wrap propagates the nil-in/nil-out
	// contract, and we depend on it at call sites.
	require.Nil(t, errw(nil))

	wrapped := errw(errors.New("underlying"))
	require.Error(t, wrapped)
	require.Contains(t, wrapped.Error(), "parquet")
}
