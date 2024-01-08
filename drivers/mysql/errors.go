package mysql

import (
	"errors"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/go-sql-driver/mysql"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// errw wraps any error from the db. It should be called at
// every interaction with the db. If err is nil, nil is returned.
// Certain errors will be wrapped in specific error types,
// e.g, errz.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case hasErrCode(err, errNumTableNotExist):
		return driver.NewNotExistError(err)
	default:
		return errz.Err(err)
	}
}

// hasErrCode returns true if err (or its cause error)
// is of type *mysql.MySQLError and err.Number equals code.
func hasErrCode(err error, code uint16) bool {
	if err == nil {
		return false
	}

	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == code
	}

	return false
}

// https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html
const (
	errNumTableNotExist = uint16(1146)
	errNumConCount      = uint16(1040)
)

func isErrTooManyConnections(err error) bool {
	return hasErrCode(err, errNumConCount)
}
