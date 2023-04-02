package errz_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/slg/lga"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/stretchr/testify/require"
)

func TestIs(t *testing.T) {
	err := errz.Wrap(sql.ErrNoRows, "wrap")

	require.Equal(t, "wrap: "+sql.ErrNoRows.Error(), err.Error())
	require.True(t, errors.Is(err, sql.ErrNoRows))
}

func TestAs(t *testing.T) {
	var originalErr error //nolint:gosimple
	originalErr = &CustomError{msg: "huzzah"}

	err := errz.Wrap(errz.Wrap(originalErr, "wrap"), "wrap")
	require.Equal(t, "wrap: wrap: huzzah", err.Error())

	var gotCustomErr *CustomError
	require.True(t, errors.As(err, &gotCustomErr))
	require.Equal(t, "huzzah", gotCustomErr.msg)

	gotUnwrap := errz.Cause(err)
	require.Equal(t, *originalErr.(*CustomError), *gotUnwrap.(*CustomError)) //nolint:errorlint
}

type CustomError struct {
	msg string
}

func (e *CustomError) Error() string {
	return e.msg
}

func TestLogError_LogValue(t *testing.T) {
	log := slogt.New(t)
	nakedErr := sql.ErrNoRows

	log.Debug("naked", lga.Err, nakedErr)

	zErr := errz.Err(nakedErr)
	log.Debug("via errz.Err", lga.Err, zErr)

	wrapErr := errz.Wrap(nakedErr, "wrap me")
	log.Debug("via errz.Wrap", lga.Err, wrapErr)
}
