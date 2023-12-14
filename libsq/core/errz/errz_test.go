package errz_test

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgt"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

func TestIs(t *testing.T) {
	err := errz.Wrap(sql.ErrNoRows, "wrap")

	require.Equal(t, "wrap: "+sql.ErrNoRows.Error(), err.Error())
	require.True(t, errors.Is(err, sql.ErrNoRows))
}

func TestWrapCauseAs(t *testing.T) {
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
	log := lgt.New(t)
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

func TestIsType(t *testing.T) {
	_, err := os.Open(stringz.Uniq32() + "-non-existing")
	require.Error(t, err)
	t.Logf("err: %T %v", err, err)

	got := errz.IsType[*fs.PathError](err)
	require.True(t, got)

	got = errz.IsType[*url.Error](err)
	require.False(t, got)
}

func TestAs(t *testing.T) {
	fp := stringz.Uniq32() + "-non-existing"
	_, err := os.Open(fp)
	require.Error(t, err)
	t.Logf("err: %T %v", err, err)

	ok, pathErr := errz.As[*fs.PathError](err)
	require.True(t, ok)
	require.NotNil(t, pathErr)
	require.Equal(t, fp, pathErr.Path)
}
