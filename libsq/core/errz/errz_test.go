package errz_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
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

func TestIsErrNotExist(t *testing.T) {
	var err error
	require.False(t, errz.IsErrNotExist(err))
	require.False(t, errz.IsErrNotExist(errz.New("huzzah")))

	var nee1 *errz.NotExistError
	require.True(t, errz.IsErrNotExist(nee1))

	var nee2 *errz.NotExistError
	require.True(t, errors.As(nee1, &nee2))

	err = errz.NotExist(errz.New("huzzah"))
	require.True(t, errz.IsErrNotExist(err))
	err = fmt.Errorf("wrap me: %w", err)
	require.True(t, errz.IsErrNotExist(err))
}

func TestIsErrNoData(t *testing.T) {
	var err error
	require.False(t, errz.IsErrNoData(err))
	require.False(t, errz.IsErrNoData(errz.New("huzzah")))

	var nde1 *errz.NoDataError
	require.True(t, errz.IsErrNoData(nde1))

	var nde2 *errz.NoDataError
	require.True(t, errors.As(nde1, &nde2))

	err = errz.NoData(errz.New("huzzah"))
	require.True(t, errz.IsErrNoData(err))
	err = fmt.Errorf("wrap me: %w", err)
	require.True(t, errz.IsErrNoData(err))

	err = errz.NoDataf("%s doesn't exist", "me")
	require.True(t, errz.IsErrNoData(err))
	require.Equal(t, "me doesn't exist", err.Error())
}
