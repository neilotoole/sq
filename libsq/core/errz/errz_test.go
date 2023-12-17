package errz_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

func TestErrEmpty(t *testing.T) {
	err := errz.New("")
	gotMsg := err.Error()
	require.Equal(t, "", gotMsg)
	gotCause := errz.UnwrapChain(err)
	require.NotNil(t, gotCause)
}

func TestErrorf(t *testing.T) {
	err := errz.Errorf("hello %s", "world")
	require.Equal(t, "hello world", err.Error())
	chain := errz.Chain(err)
	require.Len(t, chain, 1)

	err2 := errz.Errorf("wrap %w", err)
	require.Equal(t, "wrap hello world", err2.Error())
	chain2 := errz.Chain(err2)

	// chain2 should have length 3:
	// - the original "hello world";
	// - the wrapping error from fmt.Errorf to handle the %w verb;
	// - the final outer wrapper that errz.Errorf added to the fmt.Errorf error.
	require.Len(t, chain2, 3)
}

func TestUnwrapChain(t *testing.T) {
	var originalErr error //nolint:gosimple
	originalErr = &customError{msg: "huzzah"}

	err := errz.Wrap(errz.Wrap(originalErr, "wrap1"), "wrap2")
	require.Equal(t, "wrap2: wrap1: huzzah", err.Error())

	var gotCustomErr *customError
	require.True(t, errors.As(err, &gotCustomErr))
	require.Equal(t, "huzzah", gotCustomErr.msg)

	gotUnwrap := errz.UnwrapChain(err)
	require.Equal(t, *originalErr.(*customError), *gotUnwrap.(*customError)) //nolint:errorlint
}

type customError struct {
	msg string
}

func (e *customError) Error() string {
	return e.msg
}

func TestLogValue(t *testing.T) {
	log := lgt.New(t)
	nakedErr := sql.ErrNoRows

	log.Debug("naked", "err", nakedErr)

	zErr := errz.Err(nakedErr)
	log.Debug("via errz.Err", "err", zErr)

	wrapErr := errz.Wrap(nakedErr, "wrap me")
	log.Debug("via errz.Wrap", "err", wrapErr)
}

func TestHas(t *testing.T) {
	_, err := os.Open(stringz.Uniq32() + "-non-existing")
	require.Error(t, err)
	t.Logf("err: %T %v", err, err)

	got := errz.Has[*fs.PathError](err)
	require.True(t, got)

	got = errz.Has[*url.Error](err)
	require.False(t, got)

	got = errz.Has[*url.Error](nil)
	require.False(t, got)
}

func TestAs(t *testing.T) {
	fp := stringz.Uniq32() + "-non-existing"
	_, err := os.Open(fp)
	require.Error(t, err)
	t.Logf("err: %T %v", err, err)

	pathErr, ok := errz.As[*fs.PathError](err)
	require.True(t, ok)
	require.NotNil(t, pathErr)
	require.Equal(t, fp, pathErr.Path)
}

func TestIs(t *testing.T) {
	err := errz.Wrap(sql.ErrNoRows, "wrap")
	require.Equal(t, "wrap: "+sql.ErrNoRows.Error(), err.Error())
	require.True(t, errors.Is(err, sql.ErrNoRows))
}

func TestStackTrace(t *testing.T) {
	e1 := errz.New("inner")
	e2 := errz.Wrap(e1, "wrap")

	gotStacks := errz.Stacks(e2)
	require.Len(t, gotStacks, 2)

	gotFinalStack := errz.LastStack(e2)
	require.NotNil(t, gotFinalStack)
	require.Equal(t, gotStacks[len(gotStacks)-1], gotFinalStack)
}

func TestOptSkip(t *testing.T) {
	err := errz.Wrap(errz.New("inner"), "wrap1")
	chain := errz.Chain(err)
	require.Len(t, chain, 2)

	errSkip0 := errz.Err(err, errz.Skip(0))
	errSkip1 := errz.Err(err, errz.Skip(1))
	errSkip2 := errz.Err(err, errz.Skip(2))

	require.NotNil(t, errSkip0)
	require.NotNil(t, errSkip1)
	require.NotNil(t, errSkip2)
	stacks1 := errz.Stacks(errSkip1)
	require.Len(t, stacks1[0].Frames, 2)
}

type FooError struct {
	msg string
}

func (e *FooError) Error() string {
	return e.msg
}

func NewFooError(msg string) error {
	return errz.Err(&FooError{msg: msg}, errz.Skip(1))
}

func TestFooError(t *testing.T) {
	err := NewFooError("bah")
	t.Logf("err: %v", err)
	stacks := errz.Stacks(err)
	require.Len(t, stacks, 1)
	st := stacks[0]
	t.Logf("\n%+v", st.Frames)
}

//nolint:lll
func TestSprintTreeTypes(t *testing.T) {
	err := errz.Wrap(errz.Wrap(errz.New("inner"), "wrap1"), "")
	require.Equal(t, "wrap1: inner", err.Error())

	got := errz.SprintTreeTypes(err)
	require.Equal(t, "*errz.errz: *errz.errz: *errz.errz", got)

	me := errz.Combine(context.DeadlineExceeded, err, sql.ErrNoRows)
	err = errz.Wrap(me, "wrap3")
	got = errz.SprintTreeTypes(err)

	require.Equal(t, "*errz.errz: *errz.multiErr[context.deadlineExceededError, *errz.errz: *errz.errz: *errz.errz, *errors.errorString]", got)
}
