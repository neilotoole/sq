package errz

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestForeignCause(t *testing.T) {
	err := New("boo")

	cause := err.(*errz).foreignCause()
	require.Nil(t, cause)

	err = Err(context.DeadlineExceeded)
	cause = err.(*errz).foreignCause()
	require.Equal(t, context.DeadlineExceeded, cause)

	err = Err(context.DeadlineExceeded)
	err = Wrap(err, "wrap")
	cause = err.(*errz).foreignCause()
	require.Equal(t, context.DeadlineExceeded, cause)
}
