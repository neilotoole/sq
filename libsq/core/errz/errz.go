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
	"fmt"
	"go.uber.org/multierr"
	"log/slog"
)

// Err annotates err with a stack trace at the point WithStack was called.
// If err is nil, Err returns nil.
func Err(err error) error {
	if err == nil {
		return nil
	}
	return &withStack{
		err,
		callers(),
	}
}

// Append is documented by multierr.Append.
var Append = multierr.Append

// Combine is documented by multierr.Combine.
var Combine = multierr.Combine

// Errors is documented by multierr.Errors.
var Errors = multierr.Errors

// logValue return a slog.Value for err.
func logValue(err error) slog.Value {
	if err == nil {
		return slog.Value{}
	}

	c := Cause(err)
	if c == nil {
		// Shouldn't happen
		return slog.Value{}
	}

	attrs := []slog.Attr{slog.String("msg", err.Error())}
	if !errors.Is(c, err) {
		attrs = append(attrs,
			slog.String("cause", c.Error()),
			slog.String("type", fmt.Sprintf("%T", c)),
		)

		// If there's a cause c, "type" will be the type of c.
	} else {
		// If there's no cause, "type" will be the type of err.
		// It's a bit wonky, but probably the most useful thing to show.
		attrs = append(attrs, slog.String("type", fmt.Sprintf("%T", err)))
	}

	return slog.GroupValue(attrs...)
}

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

// IsErrContextDeadlineExceeded returns true if err is context.DeadlineExceeded.
func IsErrContextDeadlineExceeded(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

// Tuple returns t and err, wrapping err with errz.Err.
func Tuple[T any](t T, err error) (T, error) {
	return t, Err(err)
}

// As is a convenience wrapper around errors.As.
//
//	_, err := os.Open("non-existing")
//	ok, pathErr := errz.As[*fs.PathError](err)
//	require.True(t, ok)
//	require.Equal(t, "non-existing", pathErr.Path)
//
// Under the covers, As delegates to errors.As.
func As[E error](err error) (bool, E) {
	var target E
	if errors.As(err, &target) {
		return true, target
	}
	return false, target
}

// IsType returns true if err, or an error in its tree, if of type E.
//
//		_, err := os.Open("non-existing")
//	 isPathErr := errz.IsType[*fs.PathError](err)
//
// Under the covers, IsType uses errors.As.
func IsType[E error](err error) bool {
	var target E
	return errors.As(err, &target)
}
