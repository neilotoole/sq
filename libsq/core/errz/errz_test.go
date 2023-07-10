package errz_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

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

func TestIsErrRelationNotExist(t *testing.T) {
	var err error
	require.False(t, errz.IsErrNotExist(err))
	require.False(t, errz.IsErrNotExist(errz.New("huzzah")))

	var rne1 *errz.NotExistError
	require.True(t, errz.IsErrNotExist(rne1))

	var rne2 *errz.NotExistError
	require.True(t, errors.As(rne1, &rne2))

	err = errz.NotExist(errz.New("huzzah"))
	require.True(t, errz.IsErrNotExist(err))
	err = fmt.Errorf("wrap me: %w", err)
	require.True(t, errz.IsErrNotExist(err))
}

func TestStack(t *testing.T) {
	err := errz.New("inside")

	stacks := errz.Stack(err)
	for _, st := range stacks {
		t.Logf("%+v", st)
	}
}

func TestStack2(t *testing.T) {
	err := getPrez()
	//stacks := errz.Stack(err)
	//for _, st := range stacks {
	//	t.Logf("%+v", st)
	//}

	log := slogt.New(t)
	log.Error("huzzah", lga.Err, err)
}

func getBiz() error {
	err := getRepo()
	return errz.Wrap(err, "biz")
}

func getPrez() error {
	err := getBiz()
	return errz.Wrap(err, "prez")
}

func getDB() error {
	errDB := errors.New("some db error")
	return errDB
}

func getRepo() error {
	return getDB()
}
