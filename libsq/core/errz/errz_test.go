package errz_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/stretchr/testify/require"
)

func TestIs(t *testing.T) {
	var err error
	err = errz.Wrap(sql.ErrNoRows, "wrap")

	require.Equal(t, "wrap: "+sql.ErrNoRows.Error(), err.Error())
	require.True(t, errors.Is(err, sql.ErrNoRows))
}

func TestAs(t *testing.T) {
	var originalErr error
	originalErr = &CustomError{msg: "huzzah"}

	var err error
	err = errz.Wrap(errz.Wrap(originalErr, "wrap"), "wrap")
	require.Equal(t, "wrap: wrap: huzzah", err.Error())

	var gotCustomErr *CustomError
	require.True(t, errors.As(err, &gotCustomErr))
	require.Equal(t, "huzzah", gotCustomErr.msg)

	gotUnwrap := errz.Cause(err)
	require.Equal(t, *originalErr.(*CustomError), *gotUnwrap.(*CustomError))
}

type CustomError struct {
	msg string
}

func (e *CustomError) Error() string {
	return e.msg
}
