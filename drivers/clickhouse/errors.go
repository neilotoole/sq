package clickhouse

// This file contains error handling utilities for the ClickHouse driver.
// It provides error wrapping and classification to translate ClickHouse-specific
// errors into sq's standard error types (e.g., driver.NotExistError).
//
// ClickHouse errors are identified by numeric error codes defined in:
// https://github.com/ClickHouse/ClickHouse/blob/master/src/Common/ErrorCodes.cpp
//
// The clickhouse-go driver returns errors as *clickhouse.Exception with a Code
// field containing these error codes. This file maps relevant codes to sq's
// error taxonomy.

import (
	"errors"

	clickhouse "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
)

// errw wraps any error from the database. It should be called at every
// interaction with the database to ensure consistent error handling.
//
// Behavior:
//   - If err is nil, returns nil (no-op for success cases)
//   - If err is a ClickHouse UNKNOWN_TABLE error (code 60), wraps it in
//     driver.NotExistError so callers can use errors.Is for detection
//   - All other errors are wrapped with errz.Err for stack trace capture
//
// This function enables sq's generic error handling code to detect database-
// specific conditions without knowledge of ClickHouse internals.
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

// ClickHouse error codes from ErrorCodes.cpp.
//
// ClickHouse uses numeric error codes to identify specific error conditions.
// The full list is maintained in the ClickHouse source code:
// https://github.com/ClickHouse/ClickHouse/blob/master/src/Common/ErrorCodes.cpp
//
// Only error codes needed for sq's error classification are defined here.
// Additional codes can be added as needed for new error handling requirements.
const (
	// errCodeUnknownTable (code 60) is returned when a referenced table does
	// not exist in the database. This maps to driver.NotExistError in sq.
	//
	// Example error message from ClickHouse:
	//   "Table mydb.nonexistent doesn't exist. (UNKNOWN_TABLE)"
	errCodeUnknownTable = int32(60)
)

// hasErrCode returns true if err (or any error in its chain) is a ClickHouse
// exception with the specified error code.
//
// This function uses errors.As to unwrap the error chain, so it works correctly
// even when the ClickHouse error has been wrapped by errz.Err or other wrappers.
//
// Example usage:
//
//	if hasErrCode(err, errCodeUnknownTable) {
//	    // Handle missing table
//	}
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

// isErrUnknownTable returns true if err is a ClickHouse UNKNOWN_TABLE error
// (code 60), indicating that a referenced table does not exist.
//
// This is a convenience wrapper around hasErrCode for the most common
// "not exists" error case. It works with wrapped errors.
func isErrUnknownTable(err error) bool {
	return hasErrCode(err, errCodeUnknownTable)
}
