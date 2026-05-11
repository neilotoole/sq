// Package orshared holds Oracle error translation shared by database/sql
// wire drivers (e.g. go-ora).
package orshared

import (
	"errors"

	goora "github.com/sijms/go-ora/v2/network"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// Oracle error codes used by Wrap and HasErrCode.
const (
	ErrCodeTableNotFound     = 942 // ORA-00942: table or view does not exist
	ErrCodeInvalidIdentifier = 904 // ORA-00904: invalid identifier
)

// errCode reports the ORA-NNNNN code carried by err, or 0 if err does not
// originate from a recognised Oracle wire driver.
//
// go-ora exposes the code as the exported field network.OracleError.ErrCode
// (no Code() accessor), so a method-set type assertion does not match.
func errCode(err error) int {
	var oraErr *goora.OracleError
	if errors.As(err, &oraErr) {
		return oraErr.ErrCode
	}
	return 0
}

// Wrap converts driver errors to sq error types where applicable.
func Wrap(err error) error {
	if err == nil {
		return nil
	}

	switch errCode(err) {
	case ErrCodeTableNotFound, ErrCodeInvalidIdentifier:
		return driver.NewNotExistError(err)
	}

	return errz.Err(err)
}

// HasErrCode reports whether err is an Oracle error with the given code.
func HasErrCode(err error, code int) bool {
	if err == nil || code == 0 {
		return false
	}
	return errCode(err) == code
}

// IsErrTableNotExist reports ORA-00942.
func IsErrTableNotExist(err error) bool {
	return HasErrCode(err, ErrCodeTableNotFound)
}
