package errz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

//nolint:errorlint
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

//nolint:errorlint
func TestAppendNilToMulti(t *testing.T) {
	merr := Append(New("a"), New("b"))
	_, ok := merr.(*multiErr)
	require.True(t, ok)

	got := Append(merr, nil)
	_, ok = got.(*multiErr)
	require.True(t, ok)

	got = Append(nil, merr)
	_, ok = got.(*multiErr)
	require.True(t, ok)
}
