package errz

import (
	"errors"
)

// NotExistError indicates that a DB object, such
// as a table, does not exist.
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
