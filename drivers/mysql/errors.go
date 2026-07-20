package mysql

import (
	"errors"

	"github.com/go-sql-driver/mysql"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
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
	// errNumUnknownColumn is ER_BAD_FIELD_ERROR: "Unknown column 'X' in 'field list'".
	errNumUnknownColumn = uint16(1054)
	// errNumUnknownTable is ER_UNKNOWN_TABLE: "Unknown table 'X' in Y". MySQL 5.6
	// returns this (rather than 1146) for a missing information_schema table,
	// such as CHECK_CONSTRAINTS.
	errNumUnknownTable = uint16(1109)
)

// isMissingInfoSchemaTable reports whether err indicates that a queried
// information_schema table does not exist on this server version. MySQL 8.0+
// returns ER_NO_SUCH_TABLE (1146); MySQL 5.6 returns ER_UNKNOWN_TABLE (1109)
// for a missing information_schema table such as CHECK_CONSTRAINTS.
func isMissingInfoSchemaTable(err error) bool {
	return hasErrCode(err, errNumTableNotExist) || hasErrCode(err, errNumUnknownTable)
}

func isErrTooManyConnections(err error) bool {
	return hasErrCode(err, errNumConCount)
}
