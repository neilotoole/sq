package oracle

import (
	"errors"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// Oracle error codes currently used by the local error helpers.
const (
	errCodeTableNotFound     = 942 // ORA-00942: table or view does not exist
	errCodeInvalidIdentifier = 904 // ORA-00904: invalid identifier
)

// errw wraps Oracle errors, converting them to SQ error types.
func errw(err error) error {
	if err == nil {
		return nil
	}

	var oraErr interface{ Code() int }
	if errors.As(err, &oraErr) {
		code := oraErr.Code()

		switch code {
		case errCodeTableNotFound:
			return driver.NewNotExistError(err)
		case errCodeInvalidIdentifier:
			return driver.NewNotExistError(err)
		}
	}

	return errz.Err(err)
}

// hasErrCode checks if error matches Oracle error code.
func hasErrCode(err error, code int) bool {
	if err == nil {
		return false
	}

	var oraErr interface{ Code() int }
	if errors.As(err, &oraErr) {
		return oraErr.Code() == code
	}

	return false
}

// isErrTableNotExist checks for ORA-00942.
func isErrTableNotExist(err error) bool {
	return hasErrCode(err, errCodeTableNotFound)
}
