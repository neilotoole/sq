package errz_test

import (
	"errors"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNoData(t *testing.T) {
	booErr := errors.New("boo")
	ndErr := errz.NoData(booErr)
	st := errz.Stacks(ndErr)
	require.NotNil(t, st)

	unwrap1 := errors.Unwrap(ndErr)
	require.NotNil(t, unwrap1)
	require.Equal(t, booErr, unwrap1)
}
