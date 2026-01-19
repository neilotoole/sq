package clickhouse

import (
	"errors"

	"github.com/ClickHouse/clickhouse-go/v2"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// errw wraps any error from the db. It should be called at
// every interaction with the db. If err is nil, nil is returned.
// Certain errors will be wrapped in specific error types,
// e.g., driver.NotExistError.
func errw(err error) error {
	switch {
	case err == nil:
		return nil
	case hasErrCode(err, errCodeUnknownTable):
		return driver.NewNotExistError(err)
	default:
		return errz.Err(err)
	}
}

// ClickHouse error codes.
// See: https://github.com/ClickHouse/ClickHouse/blob/master/src/Common/ErrorCodes.cpp
const (
	// errCodeUnknownTable is returned when a referenced table doesn't exist.
	// Error message: "Table {db}.{table} doesn't exist. (UNKNOWN_TABLE)".
	errCodeUnknownTable = int32(60)
)

// hasErrCode returns true if err (or its cause error)
// is of type *clickhouse.Exception and err.Code equals code.
func hasErrCode(err error, code int32) bool {
	if err == nil {
		return false
	}

	var chErr *clickhouse.Exception
	if errors.As(err, &chErr) {
		return chErr.Code == code
	}

	return false
}

// isErrUnknownTable returns true if err is a ClickHouse error
// with code 60 (UNKNOWN_TABLE).
func isErrUnknownTable(err error) bool {
	return hasErrCode(err, errCodeUnknownTable)
}
