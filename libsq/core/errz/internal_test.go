package errz

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAlienCause(t *testing.T) {
	err := New("boo")

	cause := err.(*errz).alienCause()
	require.Nil(t, cause)

	err = Err(context.DeadlineExceeded)
	cause = err.(*errz).alienCause()
	require.Equal(t, context.DeadlineExceeded, cause)

	err = Err(context.DeadlineExceeded)
	err = Wrap(err, "wrap")
	cause = err.(*errz).alienCause()
	require.Equal(t, context.DeadlineExceeded, cause)
}
