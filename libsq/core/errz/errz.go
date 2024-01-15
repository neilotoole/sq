// Package errz is sq's error package. It annotates errors with stack traces,
// and provides functionality for working with multiple errors, and error
// chains.
//
// This package is the lovechild of Dave Cheney's pkg/errors and
// Uber's go.uber.org/multierr, and much of the code is borrowed
// from those packages.
package errz

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// Opt is a functional option. Use with [Err] or [New].
type Opt interface {
	apply(*errz)
}

// Err annotates err with a stack trace at the point Err was called.
// It is equivalent to Wrap(err, ""). If err is nil, Err returns nil.
func Err(err error, opts ...Opt) error {
	if err == nil {
		return nil
	}
	ez := &errz{stack: callers(0), error: err}
	for _, opt := range opts {
		opt.apply(ez)
	}
	return ez
}

// New returns an error with the supplied message, recording the
// stack trace at the point it was called.
func New(message string, opts ...Opt) error {
	ez := &errz{stack: callers(0), msg: message}
	for _, opt := range opts {
		opt.apply(ez)
	}
	return ez
}

// Errorf works like [fmt.Errorf], but it also records the stack trace
// at the point it was called. If the format string includes the %w verb,
// [fmt.Errorf] is first called to construct the error, and then the
// returned error is again wrapped to record the stack trace.
func Errorf(format string, args ...any) error {
	if strings.Contains(format, "%w") {
		return &errz{stack: callers(0), error: fmt.Errorf(format, args...)}
	}
	return &errz{stack: callers(0), msg: fmt.Sprintf(format, args...)}
}

// errz is our error type that does the magic.
type errz struct {
	error
	msg string
	*stack
}

// inner implements stackTracer.
func (e *errz) inner() error { return e.error }

// stackTrace implements stackTracer.
func (e *errz) stackTrace() *StackTrace {
	if e == nil || e.stack == nil {
		return nil
	}

	st := e.stack.stackTrace()
	if st != nil {
		st.Error = e
	}
	return st
}

// Error implements stdlib error interface.
func (e *errz) Error() string {
	if e.msg == "" {
		if e.error == nil {
			return ""
		}
		return e.error.Error()
	}
	if e.error == nil {
		return e.msg
	}
	return e.msg + ": " + e.error.Error()
}

// LogValue implements [slog.LogValuer]. It returns a [slog.GroupValue],
// having attributes "msg" and "type". If the error has a cause that
// from outside this package, the cause's type is included in a
// "cause" attribute.
func (e *errz) LogValue() slog.Value {
	if e == nil {
		return slog.Value{}
	}

	attrs := make([]slog.Attr, 2, 3)
	attrs[0] = slog.String("msg", e.Error())
	attrs[1] = slog.String("type", fmt.Sprintf("%T", e))

	if cause := e.alienCause(); cause != nil {
		attrs = append(attrs, slog.String("cause", fmt.Sprintf("%T", cause)))
	}

	return slog.GroupValue(attrs...)
}

// alienCause returns the first error in the chain that is
// not of type *errz, or returns nil if no such error.
func (e *errz) alienCause() error {
	if e == nil {
		return nil
	}

	inner := e.error
	for inner != nil {
		// Note: don't use errors.As here; we want the direct type assertion.
		if v, ok := inner.(*errz); ok { //nolint:errorlint
			inner = v.error
			continue
		}
		return inner
	}
	return nil
}

// Unwrap provides compatibility for Go 1.13 error chains.
func (e *errz) Unwrap() error { return e.error }

// Format implements fmt.Formatter.
func (e *errz) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			if e.error == nil {
				_, _ = io.WriteString(s, e.msg)
				e.stack.Format(s, verb)
				return
			}
			_, _ = fmt.Fprintf(s, "%+v", e.error)
			e.stack.Format(s, verb)

			return
		}
		fallthrough
	case 's':
		if e.error == nil {
			_, _ = io.WriteString(s, e.msg)
			return
		}
		_, _ = io.WriteString(s, e.Error())
	case 'q':
		if e.error == nil {
			_, _ = fmt.Fprintf(s, "{%s}", e.msg)
			return
		}
		_, _ = fmt.Fprintf(s, "{%s}", e.Error())
	}
}

// Wrap returns an error annotating err with a stack trace
// at the point Wrap is called, and the supplied message.
// If err is nil, Wrap returns nil. See also: Wrapf.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	return &errz{stack: callers(0), error: err, msg: message}
}

// Wrapf returns an error annotating err with a stack trace
// at the point Wrapf is called. Wrapf will panic if format
// includes the %w verb: use errz.Errorf for that situation.
// If err is nil, Wrapf returns nil. See also: Wrap, Errorf.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}

	if strings.Contains(format, "%w") {
		panic("errz.Wrapf does not support %w verb: use errz.Errorf instead")
	}

	return &errz{error: err, msg: fmt.Sprintf(format, args...), stack: callers(0)}
}

// UnwrapChain returns the underlying *root* cause of the error. That is
// to say, UnwrapChain returns the final non-nil error in the error chain.
// UnwrapChain returns nil if err is nil.
func UnwrapChain(err error) error {
	if err == nil {
		return nil
	}

	var cause error
	for {
		if cause = errors.Unwrap(err); cause == nil {
			break
		}
		err = cause
	}
	return err
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
// If err is nil, As returns false. See also: [errz.Has].
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

// Has returns true if err, or an error in its error tree, matches error type E.
// An error is considered a match by the rules of [errors.As]
//
//	f, err := os.Open("non-existing")
//	if errz.Has[*fs.PathError](err) {
//		// Do something
//	}
//
// If err is nil, Has returns false. See also: [errz.As].
func Has[E error](err error) bool {
	return errors.As(err, new(E))
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

// SprintTreeTypes returns a string representation of err's type tree.
// A multi-error is represented as a slice of its children.
func SprintTreeTypes(err error) string {
	if err == nil {
		return ""
	}
	errChain := Chain(err)
	var sb strings.Builder
	for i, e := range errChain {
		sb.WriteString(fmt.Sprintf("%T", e))
		if me, ok := e.(multipleErrorer); ok { //nolint:errorlint
			children := me.Unwrap()
			childText := make([]string, len(children))
			for j := range children {
				childText[j] = SprintTreeTypes(children[j])
			}
			joined := strings.Join(childText, ", ")
			sb.WriteRune('[')
			sb.WriteString(joined)
			sb.WriteRune(']')
		}

		if i < len(errChain)-1 {
			sb.WriteString(": ")
		}
	}

	return sb.String()
}
