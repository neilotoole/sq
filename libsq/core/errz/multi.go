package errz

// ACKNOWLEDGEMENT: This code is lifted from uber's multierr package.

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/samber/lo"
)

// Copyright (c) 2017-2023 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package multierr allows combining one or more errors together.
//
// # Overview
//
// Errors can be combined with the use of the Combine function.
//
//	multierr.Combine(
//		reader.Close(),
//		writer.Close(),
//		conn.Close(),
//	)
//
// If only two errors are being combined, the Append function may be used
// instead.
//
//	err = multierr.Append(reader.Close(), writer.Close())
//
// The underlying list of errors for a returned error object may be retrieved
// with the Errors function.
//
//	errors := multierr.Errors(err)
//	if len(errors) > 0 {
//		fmt.Println("The following errors occurred:", errors)
//	}
//
// # Appending from a loop
//
// You sometimes need to append into an error from a loop.
//
//	var err error
//	for _, item := range items {
//		err = multierr.Append(err, process(item))
//	}
//
// Cases like this may require knowledge of whether an individual instance
// failed. This usually requires introduction of a new variable.
//
//	var err error
//	for _, item := range items {
//		if perr := process(item); perr != nil {
//			log.Warn("skipping item", item)
//			err = multierr.Append(err, perr)
//		}
//	}
//
// multierr includes AppendInto to simplify cases like this.
//
//	var err error
//	for _, item := range items {
//		if multierr.AppendInto(&err, process(item)) {
//			log.Warn("skipping item", item)
//		}
//	}
//
// This will append the error into the err variable, and return true if that
// individual error was non-nil.
//
// See AppendInto for more information.
//
// # Deferred Functions
//
// Go makes it possible to modify the return value of a function in a defer
// block if the function was using named returns. This makes it possible to
// record resource cleanup failures from deferred blocks.
//
//	func sendRequest(req Request) (err error) {
//		conn, err := openConnection()
//		if err != nil {
//			return err
//		}
//		defer func() {
//			err = multierr.Append(err, conn.Close())
//		}()
//		// ...
//	}
//
// multierr provides the Invoker type and AppendInvoke function to make cases
// like the above simpler and obviate the need for a closure. The following is
// roughly equivalent to the example above.
//
//	func sendRequest(req Request) (err error) {
//		conn, err := openConnection()
//		if err != nil {
//			return err
//		}
//		defer multierr.AppendInvoke(&err, multierr.Close(conn))
//		// ...
//	}
//
// See AppendInvoke and Invoker for more information.
//
// NOTE: If you're modifying an error from inside a defer, you MUST use a named
// return value for that function.
//
// # Advanced Usage
//
// Errors returned by Combine and Append MAY implement the following
// interface.
//
//	type errorGroup interface {
//		// Returns a slice containing the underlying list of errors.
//		//
//		// This slice MUST NOT be modified by the caller.
//		Errors() []error
//	}
//
// Note that if you need access to list of errors behind a multierr error, you
// should prefer using the Errors function. That said, if you need cheap
// read-only access to the underlying errors slice, you can attempt to cast
// the error to this interface. You MUST handle the failure case gracefully
// because errors returned by Combine and Append are not guaranteed to
// implement this interface.
//
//	var errors []error
//	group, ok := err.(errorGroup)
//	if ok {
//		errors = group.Errors()
//	} else {
//		errors = []error{err}
//	}

var (
	// Separator for single-line error messages.
	_singlelineSeparator = []byte("; ")

	// Prefix for multi-line messages.
	_multilinePrefix = []byte("the following errors occurred:")

	// Prefix for the first and following lines of an item in a list of
	// multi-line error messages.
	//
	// For example, if a single item is:
	//
	// 	foo
	// 	bar
	//
	// It will become,
	//
	// 	 -  foo
	// 	    bar
	_multilineSeparator = []byte("\n -  ")
	_multilineIndent    = []byte("    ")
)

// _bufferPool is a pool of bytes.Buffers.
var _bufferPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

// Errors returns a slice containing zero or more errors that the supplied
// error is composed of. If the error is nil, a nil slice is returned.
//
//	err := multierr.Append(r.Close(), w.Close())
//	errors := multierr.Errors(err)
//
// If the error is not composed of other errors, the returned slice contains
// just the error that was passed in.
//
// Callers of this function are free to modify the returned slice.
func Errors(err error) []error {
	return extractErrors(err)
}

// multiErr is an error that holds one or more errors.
//
// An instance of this is guaranteed to be non-empty and flattened. That is,
// none of the errors inside multiErr are other multiErrors.
//
// multiErr formats to a semicolon delimited list of error messages with
// %v and with a more readable multi-line format with %+v.
type multiErr struct { //nolint:errname
	*stack
	errors     []error
	copyNeeded atomic.Bool
}

// inner implements stackTracer.
func (merr *multiErr) inner() error { return nil }

// stackTrace implements stackTracer.
func (merr *multiErr) stackTrace() *StackTrace {
	if merr == nil || merr.stack == nil {
		return nil
	}

	st := merr.stack.stackTrace()
	if st != nil {
		st.Error = merr
	}
	return st
}

// Errors returns the list of underlying errors.
//
// This slice MUST NOT be modified.
func (merr *multiErr) Errors() []error {
	if merr == nil {
		return nil
	}
	return merr.errors
}

func (merr *multiErr) Error() string {
	if merr == nil {
		return ""
	}

	buff, _ := _bufferPool.Get().(*bytes.Buffer)
	buff.Reset()

	merr.writeSingleline(buff)

	result := buff.String()
	_bufferPool.Put(buff)
	return result
}

// LogValue implements slog.LogValuer.
func (merr *multiErr) LogValue() slog.Value {
	if merr == nil {
		return slog.Value{}
	}

	attrs := make([]slog.Attr, 4)
	attrs[0] = slog.String("msg", merr.Error())
	attrs[1] = slog.String("type", fmt.Sprintf("%T", merr))
	attrs[2] = slog.String("tree", SprintTreeTypes(merr))
	errs := lo.Map(merr.Errors(), func(err error, i int) string { return err.Error() })
	attrs[3] = slog.Any("errors", errs)
	return slog.GroupValue(attrs...)
}

func (merr *multiErr) Format(f fmt.State, c rune) {
	if c == 'v' && f.Flag('+') {
		merr.writeMultiline(f)
	} else {
		merr.writeSingleline(f)
	}
}

func (merr *multiErr) writeSingleline(w io.Writer) {
	first := true
	for _, item := range merr.errors {
		if first {
			first = false
		} else {
			_, _ = w.Write(_singlelineSeparator)
		}
		_, _ = io.WriteString(w, item.Error())
	}
}

func (merr *multiErr) writeMultiline(w io.Writer) {
	_, _ = w.Write(_multilinePrefix)
	for _, item := range merr.errors {
		_, _ = w.Write(_multilineSeparator)
		writePrefixLine(w, _multilineIndent, fmt.Sprintf("%+v", item))
	}
}

// Writes s to the writer with the given prefix added before each line after
// the first.
func writePrefixLine(w io.Writer, prefix []byte, s string) {
	first := true
	for len(s) > 0 {
		if first {
			first = false
		} else {
			_, _ = w.Write(prefix)
		}

		idx := strings.IndexByte(s, '\n')
		if idx < 0 {
			idx = len(s) - 1
		}

		_, _ = io.WriteString(w, s[:idx+1])
		s = s[idx+1:]
	}
}

type inspectResult struct {
	// Number of top-level non-nil errors
	Count int

	// Total number of errors including multiErrors
	Capacity int

	// Index of the first non-nil error in the list. Value is meaningless if
	// Count is zero.
	FirstErrorIdx int

	// Whether the list contains at least one multiErr
	ContainsMultiError bool
}

// Inspects the given slice of errors so that we can efficiently allocate
// space for it.
func inspect(errs []error) (res inspectResult) {
	first := true
	for i, err := range errs {
		if err == nil {
			continue
		}

		res.Count++
		if first {
			first = false
			res.FirstErrorIdx = i
		}

		if merr, ok := err.(*multiErr); ok { //nolint:errorlint
			res.Capacity += len(merr.errors)
			res.ContainsMultiError = true
		} else {
			res.Capacity++
		}
	}
	return res
}

// fromSlice converts the given list of errors into a single error.
func fromSlice(errs []error) error {
	// Don't pay to inspect small slices.
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}

	res := inspect(errs)
	switch res.Count {
	case 0:
		return nil
	case 1:
		// only one non-nil entry
		return errs[res.FirstErrorIdx]
	case len(errs):
		if !res.ContainsMultiError {
			// Error list is flat. Make a copy of it
			// Otherwise "errors" escapes to the heap
			// unconditionally for all other cases.
			// This lets us optimize for the "no errors" case.
			out := append(([]error)(nil), errs...)
			return &multiErr{errors: out, stack: callers(1)}
		}
	}

	nonNilErrs := make([]error, 0, res.Capacity)
	for _, err := range errs[res.FirstErrorIdx:] {
		if err == nil {
			continue
		}

		if nested, ok := err.(*multiErr); ok { //nolint:errorlint
			nonNilErrs = append(nonNilErrs, nested.errors...)
		} else {
			nonNilErrs = append(nonNilErrs, err)
		}
	}

	return &multiErr{errors: nonNilErrs, stack: callers(0)}
}

// Combine combines the passed errors into a single error.
//
// If zero arguments were passed or if all items are nil, a nil error is
// returned.
//
//	Combine(nil, nil)  // == nil
//
// If only a single error was passed, it is returned as-is.
//
//	Combine(err)  // == err
//
// Combine skips over nil arguments so this function may be used to combine
// together errors from operations that fail independently of each other.
//
//	multierr.Combine(
//		reader.Close(),
//		writer.Close(),
//		pipe.Close(),
//	)
//
// If any of the passed errors is a multierr error, it will be flattened along
// with the other errors.
//
//	multierr.Combine(multierr.Combine(err1, err2), err3)
//	// is the same as
//	multierr.Combine(err1, err2, err3)
//
// The returned error formats into a readable multi-line error message if
// formatted with %+v.
//
//	fmt.Sprintf("%+v", multierr.Combine(err1, err2))
func Combine(errs ...error) error {
	return fromSlice(errs)
}

// Append appends the given errors together. Either value may be nil. The
// value of the returned error's [error.Error] method is a semicolon-delimited
// list of the non-nil errors' messages. This is different from the behavior
// of stdlib [errors.Join], which separates errors with newlines. But, we choose
// to use semicolons because it's more readable in logs and error messages.
//
// This function is a specialization of Combine for the common case where
// there are only two errors.
//
//	err = multierr.Append(reader.Close(), writer.Close())
//
// The following pattern may also be used to record failure of deferred
// operations without losing information about the original error.
//
//	func doSomething(..) (err error) {
//		f := acquireResource()
//		defer func() {
//			err = multierr.Append(err, f.Close())
//		}()
//
// Note that the variable MUST be a named return to append an error to it from
// the defer statement.
//
//nolint:errorlint
func Append(left, right error) error {
	switch {
	case left == nil && right == nil:
		return nil
	case left == nil:
		switch right := right.(type) {
		case *multiErr, *errz:
			return right
		default:
			// It's not an errz, so we  need to wrap it.
			return &errz{stack: callers(0), error: right}
		}
	case right == nil:
		switch left := left.(type) {
		case *multiErr, *errz:
			return left
		default:
			// It's not an errz, so we  need to wrap it.
			return &errz{stack: callers(0), error: left}
		}
	}

	if _, ok := right.(*multiErr); !ok {
		if l, ok := left.(*multiErr); ok && !l.copyNeeded.Swap(true) {
			// Common case where the error on the left is constantly being
			// appended to.
			errs := append(l.errors, right) //nolint:gocritic
			return &multiErr{errors: errs, stack: callers(0)}
		} else if !ok {
			// Both errors are single errors.
			return &multiErr{errors: []error{left, right}, stack: callers(0)}
		}
	}

	// Either right or both, left and right, are multiErrors. Rely on usual
	// expensive logic.
	errs := [2]error{left, right}
	return fromSlice(errs[0:])
}

// Unwrap returns a list of errors wrapped by this multierr.
func (merr *multiErr) Unwrap() []error {
	return merr.Errors()
}

type multipleErrorer interface {
	Unwrap() []error
}

func extractErrors(err error) []error {
	if err == nil {
		return nil
	}

	// check if the given err is an unwrappable error that
	// implements multipleErrorer interface.
	eg, ok := err.(multipleErrorer) //nolint:errorlint
	if !ok {
		return []error{err}
	}

	return append(([]error)(nil), eg.Unwrap()...)
}

// Every compares every error in the given err against the given target error
// using errors.Is, and returns true only if every comparison returned true.
func Every(err, target error) bool {
	for _, e := range extractErrors(err) {
		if !errors.Is(e, target) {
			return false
		}
	}
	return true
}
