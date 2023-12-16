package errz

// These multi-error functions delegate to go.uber.org/multierr. That
// package was in use before stdlib introduced the errors.Join function.
// It's possible, maybe even desirable, to refactor these functions
// to use stdlib errors instead.

import "go.uber.org/multierr"

// Append appends the given errors together. Either value may be nil.
//
// This function is a specialization of Combine for the common case where
// there are only two errors.
//
//	err = errz.Append(reader.Close(), writer.Close())
//
// The following pattern may also be used to record failure of deferred
// operations without losing information about the original error.
//
//	func doSomething(..) (err error) {
//		f := acquireResource()
//		defer func() {
//			err = errz.Append(err, f.Close())
//		}()
//
// Note that the variable MUST be a named return to append an error to it from
// the defer statement.
func Append(left error, right error) error {
	switch {
	case left == nil && right == nil:
		return nil
	case left == nil:
		if _, ok := right.(*errz); !ok {
			// It's not an errz, so we  need to wrap it.
			return &errz{stack: callers(), error: right}
		}
		return right
	case right == nil:
		if _, ok := left.(*errz); !ok {
			// It's not an errz, so we  need to wrap it.
			return &errz{stack: callers(), error: left}
		}
		return left
	}

	if me := multierr.Append(left, right); me == nil {
		return nil
	} else {
		return &errz{stack: callers(), error: me}
	}

}

// Combine combines the passed errors into a single error.
//
// If zero arguments were passed or if all items are nil, a nil error is
// returned.
//
//	Combine(nil, nil)  // == nil
//
// If only a single error was passed, it is returned as-is if it's already
// an errz error; otherwise, it is wrapped before return.
//
//	Combine(err)  // == err
//
// Combine skips over nil arguments so this function may be used to combine
// together errors from operations that fail independently of each other.
//
//	errz.Combine(
//		reader.Close(),
//		writer.Close(),
//		pipe.Close(),
//	)
//
// If any of the passed errors is a multierr error, it will be flattened along
// with the other errors.
//
//	errz.Combine(errz.Combine(err1, err2), err3)
//	// is the same as
//	errz.Combine(err1, err2, err3)
//
// The returned error formats into a readable multi-line error message if
// formatted with %+v.
//
//	fmt.Sprintf("%+v", errz.Combine(err1, err2))
func Combine(errors ...error) error {
	switch len(errors) {
	case 0:
		return nil
	case 1:
		if errors[0] == nil {
			return nil
		}

		if _, ok := errors[0].(*errz); ok {
			// It's already an errz, so we don't need to wrap it.
			return errors[0]
		}

		return &errz{stack: callers(), error: errors[0]}
	}

	if me := multierr.Combine(errors...); me == nil {
		return nil
	} else {
		return &errz{stack: callers(), error: me}
	}
}

// Errors returns a slice containing zero or more errors that the supplied
// error is composed of. If the error is nil, a nil slice is returned.
//
//	err := errz.Append(r.Close(), w.Close())
//	errors := errz.Errors(err)
//
// If the error is not composed of other errors, the returned slice contains
// just the error that was passed in.
//
// Callers of this function are free to modify the returned slice.
func Errors(err error) []error {
	if err == nil {
		return nil
	}

	if me, ok := err.(multipleErrors); ok {
		return me.Unwrap()
	}

	ez, ok := err.(*errz)
	if !ok {
		return multierr.Errors(err)
	}

	// It's an errz, so let's see what's underneath.
	alien := ez.alienCause()
	if alien == nil {
		// It's not an alien error, it's just a pure errz error.
		// It can't be a multi error.
		return []error{err}
	}

	// It's a foreign error, so we let multierr take care of it.
	return multierr.Errors(alien)
}

type multipleErrors interface {
	Unwrap() []error
}
