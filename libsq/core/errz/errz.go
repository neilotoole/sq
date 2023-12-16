// Package errz is sq's error package. It exists to combine
// functionality from several error packages, including
// annotating errors with stack trace. Most of it comes
// from pkg/errors.
//
// At some point this package may become redundant, particularly in
// light of the proposed stdlib multiple error support:
// https://github.com/golang/go/issues/53435
package errz

import (
	"context"
	"errors"
	"go.uber.org/multierr"
)

// Err annotates err with a stack trace at the point Err was called.
// If err is nil, Err returns nil.
func Err(err error) error {
	if err == nil {
		return nil
	}
	return &errz{
		err,
		"",
		callers(),
	}
}

// Append is documented by multierr.Append.
var Append = multierr.Append

// Combine is documented by multierr.Combine.
var Combine = multierr.Combine

// Errors is documented by multierr.Errors.
var Errors = multierr.Errors

// IsErrContext returns true if err is context.Canceled or context.DeadlineExceeded.
func IsErrContext(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	return false
}

// Return returns t with err wrapped via [errz.Err].
// This is useful for the common case of returning a value and
// an error from a function.
//
//	written, err = errz.Return(io.Copy(w, r))
func Return[T any](t T, err error) (T, error) {
	return t, Err(err)
}

// As is a convenience wrapper around errors.As.
//
//	_, err := os.Open("non-existing")
//	pathErr, ok := errz.As[*fs.PathError](err)
//	require.True(t, ok)
//	require.Equal(t, "non-existing", pathErr.Path)
//
// If err is nil, As returns false.
func As[E error](err error) (E, bool) {
	var target E
	if err == nil {
		return target, false
	}

	if errors.As(err, &target) {
		return target, true
	}
	return target, false
}

// Has returns true if err, or an error in its error tree, if of type E.
//
//		_, err := os.Open("non-existing")
//	 isPathErr := errz.Has[*fs.PathError](err)
//
// If err is nil, Has returns false.
func Has[E error](err error) bool {
	if err == nil {
		return false
	}
	var target E
	return errors.As(err, &target)
}

// Chain returns a slice of all the errors in err's tree.
func Chain(err error) []error {
	if err == nil {
		return nil
	}

	var errs []error
	for err != nil {
		errs = append(errs, err)
		err = errors.Unwrap(err)
	}

	return errs
}
