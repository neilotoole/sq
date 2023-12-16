package errz

// ACKNOWLEDGEMENT: The code in this file has its origins
// in Dave Cheney's pkg/errors.

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
)

// New returns an error with the supplied message, recording the
// stack trace at the point it was called.
func New(message string) error {
	return &errz{stack: callers(), msg: message}
}

// Errorf works like [fmt.Errorf], but it also records the stack trace
// at the point it was called. If the format string includes the %w verb,
// [fmt.Errorf] is first called to construct the error, and then the
// returned error is again wrapped to record the stack trace.
func Errorf(format string, args ...any) error {
	if strings.Contains(format, "%w") {
		return &errz{stack: callers(), error: fmt.Errorf(format, args...)}
	}
	return &errz{stack: callers(), msg: fmt.Sprintf(format, args...)}
}

// errz is the error implementation used by this package.
type errz struct {
	error
	msg string
	*stack
}

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

// LogValue implements slog.LogValuer. It returns a slog.GroupValue,
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

	if cause := e.foreignCause(); cause != nil {
		attrs = append(attrs, slog.String("cause", fmt.Sprintf("%T", cause)))
	}

	return slog.GroupValue(attrs...)
}

// foreignCause returns the first error in the chain that is
// not of type *errz, or returns nil if no such error.
func (e *errz) foreignCause() error {
	if e == nil {
		return nil
	}

	inner := e.error
	for inner != nil {
		// Note: don't use errors.As here; we want the direct type assertion.
		if v, ok := inner.(*errz); ok {
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
			} else {
				_, _ = fmt.Fprintf(s, "%+v", e.error)
				e.stack.Format(s, verb)
			}
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

	return &errz{stack: callers(), error: err, msg: message}
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

	return &errz{error: err, msg: fmt.Sprintf(format, args...), stack: callers()}
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
