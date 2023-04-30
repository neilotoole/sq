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
	"path/filepath"

	"go.uber.org/multierr"

	"golang.org/x/exp/slog"
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

	msgAttr := slog.String("msg", err.Error())
	causeAttr := slog.String("cause", c.Error())
	typeAttr := slog.String("type", fmt.Sprintf("%T", c))

	if ws, ok := err.(*withStack); ok { //nolint:errorlint
		st := ws.stack.StackTrace()

		if len(st) > 0 {
			f := st[0]
			file := f.file()
			funcName := f.name()
			if funcName != unknown {
				fp := filepath.Join(filepath.Base(filepath.Dir(file)), filepath.Base(file))
				return slog.GroupValue(
					msgAttr,
					causeAttr,
					typeAttr,
					slog.String("func", funcName),
					slog.String("source", fmt.Sprintf("%s:%d", fp, f.line())),
				)
			}
		}
	}

	return slog.GroupValue(msgAttr, causeAttr, typeAttr)
}

// IsErrContextDeadlineExceeded returns true if err is context.DeadlineExceeded.
func IsErrContextDeadlineExceeded(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}
