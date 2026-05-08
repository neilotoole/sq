// Package orshared holds Oracle error translation shared by database/sql
// wire drivers (e.g. go-ora).
package orshared

import (
	"errors"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// Oracle error codes used by Wrap and HasErrCode.
const (
	ErrCodeTableNotFound     = 942 // ORA-00942: table or view does not exist
	ErrCodeInvalidIdentifier = 904 // ORA-00904: invalid identifier
)

// Wrap converts driver errors to sq error types where applicable.
func Wrap(err error) error {
	if err == nil {
		return nil
	}

	var oraErr interface{ Code() int }
	if errors.As(err, &oraErr) {
		code := oraErr.Code()

		switch code {
		case ErrCodeTableNotFound, ErrCodeInvalidIdentifier:
			return driver.NewNotExistError(err)
		}
	}

	return errz.Err(err)
}

// HasErrCode reports whether err is an Oracle error with the given code.
func HasErrCode(err error, code int) bool {
	if err == nil {
		return false
	}

	var oraErr interface{ Code() int }
	if errors.As(err, &oraErr) {
		return oraErr.Code() == code
	}

	return false
}

// IsErrTableNotExist reports ORA-00942.
func IsErrTableNotExist(err error) bool {
	return HasErrCode(err, ErrCodeTableNotFound)
}
