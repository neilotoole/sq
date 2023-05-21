package errz

import (
	"errors"
)

// RelationNotExistError indicates that a relation, such
// as a table, does not exist.
type RelationNotExistError struct {
	error
}

// Unwrap satisfies the stdlib errors.Unwrap function.
func (e *RelationNotExistError) Unwrap() error { return e.error }

// RelationNotExist returns a RelationNotExistError, or nil.
func RelationNotExist(err error) error {
	if err == nil {
		return nil
	}
	return &RelationNotExistError{error: Err(err)}
}

// IsErrRelationNotExist returns true if err is non-nil and
// err is or contains RelationNotExistError.
func IsErrRelationNotExist(err error) bool {
	if err == nil {
		return false
	}
	var e *RelationNotExistError
	return errors.As(err, &e)
}
