// Package errz provides simple error handling primitives.
//
// FIXME: update docs
//
// The traditional error handling idiom in Go is roughly akin to
//
//	if err != nil {
//	        return err
//	}
//
// which when applied recursively up the call stack results in error reports
// without context or debugging information. The errors package allows
// programmers to add context to the failure path in their code in a way
// that does not destroy the original value of the error.
//
// # Adding context to an error
//
// The errors.Wrap function returns a new error that adds context to the
// original error by recording a stack trace at the point Wrap is called,
// together with the supplied message. For example
//
//	_, err := ioutil.ReadAll(r)
//	if err != nil {
//	        return errors.Wrap(err, "read failed")
//	}
//
// If additional control is required, the errors.WithStack and
// errors.WithMessage functions destructure errors.Wrap into its component
// operations: annotating an error with a stack trace and with a message,
// respectively.
//
// # Retrieving the cause of an error
//
// Using errors.Wrap constructs a stack of errors, adding context to the
// preceding error. Depending on the nature of the error it may be necessary
// to reverse the operation of errors.Wrap to retrieve the original error
// for inspection. Any error value which implements this interface
//
//	type causer interface {
//	        UnwrapChain() error
//	}
//
// can be inspected by errors.UnwrapChain. errors.UnwrapChain will recursively retrieve
// the topmost error that does not implement causer, which is assumed to be
// the original cause. For example:
//
//	switch err := errors.UnwrapChain(err).(type) {
//	case *MyError:
//	        // handle specifically
//	default:
//	        // unknown error
//	}
//
// Although the causer interface is not exported by this package, it is
// considered a part of its stable public interface.
//
// # Formatted printing of errors
//
// All error values returned from this package implement fmt.Formatter and can
// be formatted by the fmt package. The following verbs are supported:
//
//	%s    print the error. If the error has a UnwrapChain it will be
//	      printed recursively.
//	%v    see %s
//	%+v   extended format. Each Frame of the error's StackTrace will
//	      be printed in detail.
//
// # Retrieving the stack trace of an error or wrapper
//
// New, Errorf, Wrap, and Wrapf record a stack trace at the point they are
// invoked. This information can be retrieved with the following interface:
//
//	type stackTracer interface {
//	        StackTrace() errors.StackTrace
//	}
//
// The returned errors.StackTrace type is defined as
//
//	type StackTrace []Frame
//
// The Frame type represents a call site in the stack trace. Frame supports
// the fmt.Formatter interface that can be used for printing information about
// the stack trace of this error. For example:
//
//	if err, ok := err.(stackTracer); ok {
//	        for _, f := range err.StackTrace() {
//	                fmt.Printf("%+s:%d\n", f, f)
//	        }
//	}
//
// Although the stackTracer interface is not exported by this package, it is
// considered a part of its stable public interface.
//
// See the documentation for Frame.Format for more details.
package errz

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
)

// New returns an error with the supplied message.
// New also records the stack trace at the point it was called.
func New(message string) error {
	return &withStack{
		msg:   message,
		stack: callers(),
	}
}

// Errorf formats according to a format specifier and returns the string
// as a value that satisfies error.
// Errorf also records the stack trace at the point it was called.
func Errorf(format string, args ...any) error {
	return &withStack{
		msg:   fmt.Sprintf(format, args...),
		stack: callers(),
	}
}

//
//// fundamental is an error that has a message and a stack, but no caller.
//type fundamental struct {
//	msg string
//	*stack
//}
//
//func (f *fundamental) Error() string { return f.msg }
//
//func (f *fundamental) Format(s fmt.State, verb rune) {
//	switch verb {
//	case 'v':
//		if s.Flag('+') {
//			_, _ = io.WriteString(s, f.msg)
//			f.stack.Format(s, verb)
//			return
//		}
//		fallthrough
//	case 's':
//		_, _ = io.WriteString(s, f.msg)
//	case 'q':
//		_, _ = fmt.Fprintf(s, "{%s}", f.msg)
//	}
//}
//
//var _ StackTracer = (*fundamental)(nil)
//
//// StackTrace implements StackTracer.
//func (f *fundamental) StackTrace() *StackTrace {
//	if f == nil || f.stack == nil {
//		return nil
//	}
//
//	st := f.stack.stackTrace()
//	if st != nil {
//		st.Error = f
//	}
//	return st
//}
//
//// LogValue implements slog.LogValuer.
//func (f *fundamental) LogValue() slog.Value {
//	return logValue(f)
//}

type withStack struct {
	error
	msg string
	*stack
}

var _ StackTracer = (*withStack)(nil)

// StackTrace implements StackTracer.
// REVISIT: consider making StackTrace private, or removing
// it in favor of the Stack function.
func (w *withStack) StackTrace() *StackTrace {
	if w == nil || w.stack == nil {
		return nil
	}

	st := w.stack.stackTrace()
	if st != nil {
		st.Error = w
	}
	return st
}

// Error implements stdlib error interface.
func (w *withStack) Error() string {
	if w.msg == "" {
		if w.error == nil {
			return ""
		}
		return w.error.Error()
	}
	if w.error == nil {
		return w.msg
	}
	return w.msg + ": " + w.error.Error()
}

// LogValue implements slog.LogValuer.
func (w *withStack) LogValue() slog.Value {
	if w == nil {
		return slog.Value{}
	}

	attrs := make([]slog.Attr, 2, 4)
	attrs[0] = slog.String("msg", w.Error())
	attrs[1] = slog.String("type", fmt.Sprintf("%T", w))

	// If there's a wrapped error, "cause" and "type" will be
	// for that wrapped error.
	if w.error != nil {
		attrs[1] = slog.String("cause", w.error.Error())
		attrs = append(attrs, slog.String("type", fmt.Sprintf("%T", w.error)))
	} else {
		// If there's no wrapped error, "type" will be the type of w.
		attrs[1] = slog.String("type", fmt.Sprintf("%T", w))
	}

	return slog.GroupValue(attrs...)
}

// UnwrapChain returns the underlying *root* cause of the error, if possible.
//
// Deprecated: get rid of UnwrapChain in favor of errors.Unwrap.
func (w *withStack) Cause() error { return w.error }

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *withStack) Unwrap() error { return w.error }

//func (f *fundamental) Format(s fmt.State, verb rune) {
//	switch verb {
//	case 'v':
//		if s.Flag('+') {
//			_, _ = io.WriteString(s, f.msg)
//			f.stack.Format(s, verb)
//			return
//		}
//		fallthrough
//	case 's':
//		_, _ = io.WriteString(s, f.msg)
//	case 'q':
//		_, _ = fmt.Fprintf(s, "{%s}", f.msg)
//	}
//}

func (w *withStack) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			if w.error == nil {
				_, _ = io.WriteString(s, w.msg)
				w.stack.Format(s, verb)
				return
			} else {
				_, _ = fmt.Fprintf(s, "%+v", w.Cause())
				w.stack.Format(s, verb)
			}
			return
		}
		fallthrough
	case 's':
		if w.error == nil {
			_, _ = io.WriteString(s, w.msg)
			return
		}
		_, _ = io.WriteString(s, w.Error())
	case 'q':
		if w.error == nil {
			_, _ = fmt.Fprintf(s, "{%s}", w.msg)
			return
		}
		_, _ = fmt.Fprintf(s, "{%s}", w.Error())
	}
}

// Wrap returns an error annotating err with a stack trace
// at the point Wrap is called, and the supplied message.
// If err is nil, Wrap returns nil.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}

	return &withStack{
		err,
		message,
		callers(),
	}
}

// Wrapf returns an error annotating err with a stack trace
// at the point Wrapf is called, and the format specifier.
// If err is nil, Wrapf returns nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	//err = &withMessage{
	//	cause: err,
	//	msg:   fmt.Sprintf(format, args...),
	//}
	return &withStack{
		err,
		fmt.Sprintf(format, args...),
		callers(),
	}
}

//
//type withMessage struct { //nolint:errname
//	cause error
//	msg   string
//}
//
//func (w *withMessage) Error() string { return w.msg + ": " + w.cause.Error() }
//func (w *withMessage) UnwrapChain() error  { return w.cause }
//
//// LogValue implements slog.LogValuer.
//func (w *withMessage) LogValue() slog.Value {
//	return logValue(w)
//}
//
//// Unwrap provides compatibility for Go 1.13 error chains.
//func (w *withMessage) Unwrap() error { return w.cause }
//
//func (w *withMessage) Format(s fmt.State, verb rune) {
//	switch verb {
//	case 'v':
//		if s.Flag('+') {
//			_, _ = fmt.Fprintf(s, "%+v\n", w.UnwrapChain())
//			_, _ = io.WriteString(s, w.msg)
//			return
//		}
//		fallthrough
//	case 's', 'q':
//		_, _ = io.WriteString(s, w.Error())
//	}
//}

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
