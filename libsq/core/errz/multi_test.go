package errz

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppend_stdlib_errors(t *testing.T) {
	err1 := errors.New("err1")
	err2 := errors.New("err2")
	errs := Errors(err1)
	require.Equal(t, []error{err1}, errs)

	appendErr := Append(err1, err2)
	errs = Errors(appendErr)
	require.Len(t, errs, 2)
	require.Equal(t, []error{err1, err2}, errs)
	t.Logf("%v", appendErr)
	t.Logf("%+v", appendErr)

	stacks := Stacks(appendErr)
	require.NotNil(t, stacks)
	require.Len(t, stacks, 1)
	st := stacks[0]
	require.NotNil(t, st)
	t.Logf("%+v", st.Frames)

	appendErr = Append(nil, nil)
	require.Nil(t, appendErr)

	appendErr = Append(err1, nil)
	require.NotNil(t, appendErr)
	errs = Errors(appendErr)
	require.Len(t, errs, 1)
	gotErr1 := errs[0]
	_, ok := gotErr1.(*errz) //nolint:errorlint
	require.True(t, ok)
	gotErr1Unwrap := errors.Unwrap(gotErr1)
	require.Equal(t, err1, gotErr1Unwrap)
}

func TestAppend_errz(t *testing.T) {
	err1 := New("err1")
	err2 := New("err2")
	errs := Errors(err1)
	require.Equal(t, []error{err1}, errs)

	appendErr := Append(err1, err2)
	errs = Errors(appendErr)
	require.Equal(t, []error{err1, err2}, errs)

	stacks := Stacks(appendErr)
	require.NotNil(t, stacks)
	require.Len(t, stacks, 1)
	st := stacks[0]
	require.NotNil(t, st)
	t.Logf("%+v", st.Frames)
}
