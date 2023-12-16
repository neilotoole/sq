package errz

import (
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestMultiErrors_stdlib_errors(t *testing.T) {
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
}

func TestMultiErrors_errz(t *testing.T) {
	err1 := New("err1")
	err2 := New("err2")
	errs := Errors(err1)
	require.Equal(t, []error{err1}, errs)

	appendErr := Append(err1, err2)
	errs = Errors(appendErr)
	require.Equal(t, []error{err1, err2}, errs)
	t.Logf("%v", appendErr)
	t.Logf("%+v", appendErr)

	stacks := Stacks(appendErr)
	require.NotNil(t, stacks)
	require.Len(t, stacks, 1)
	st := stacks[0]
	require.NotNil(t, st)
	t.Logf("%+v", st.Frames)
}
