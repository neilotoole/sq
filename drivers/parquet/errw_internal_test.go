package parquet

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrw(t *testing.T) {
	err := errw(errors.New("underlying error"))
	require.Error(t, err)
}
