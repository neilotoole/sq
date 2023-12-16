package errz

import (
	"errors"
)

// NotExistError indicates that a DB object, such
// as a table, does not exist.
//
// REVISIT: Consider moving NotExistError to libsq/driver?
type NotExistError struct {
	error
}

// Unwrap satisfies the stdlib errors.Unwrap function.
func (e *NotExistError) Unwrap() error { return e.error }

// NotExist returns a NotExistError, or nil.
func NotExist(err error) error {
	if err == nil {
		return nil
	}
	return &NotExistError{error: Err(err)}
}

// IsErrNotExist returns true if err is non-nil and
// err is or contains NotExistError.
func IsErrNotExist(err error) bool {
	if err == nil {
		return false
	}
	var e *NotExistError
	return errors.As(err, &e)
}

// NoDataError indicates that there's no data, e.g. an empty document.
// This is subtly different to NotExistError, which would indicate that
// the document doesn't exist.
//
// REVISIT: Consider moving NoDataError to libsq/driver?
// REVISIT: Consider renaming NoDataError to EmptyDataError?
type NoDataError struct {
	error
}

// Unwrap satisfies the stdlib errors.Unwrap function.
func (e *NoDataError) Unwrap() error { return e.error }

// NoData returns a NoDataError, or nil.
func NoData(err error) error {
	if err == nil {
		return nil
	}
	return &NoDataError{error: Err(err)}
}

// NoDataf returns a NoDataError.
func NoDataf(format string, args ...any) error {
	return &NoDataError{error: Errorf(format, args...)}
}

// IsErrNoData returns true if err is non-nil and
// err is or contains NoDataError.
func IsErrNoData(err error) bool {
	if err == nil {
		return false
	}
	var e *NoDataError
	return errors.As(err, &e)
}
