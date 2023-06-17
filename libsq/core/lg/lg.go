// Package lg contains utility functions for working with slog.
// It implements the slog.NewContext and slog.FromContext funcs
// that have recently been zapped from the slog proposal. I think you
// had it right the first time, Go team. Hopefully this package is short-lived
// and those funcs are put back.
package lg

import (
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"golang.org/x/exp/slog"
)

type contextKey struct{}

// NewContext returns a context that contains the given Logger.
// Use FromContext to retrieve the Logger.
func NewContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext returns the Logger stored in ctx by NewContext,
// or the Discard logger if there is none.
func FromContext(ctx context.Context) *slog.Logger {
	v := ctx.Value(contextKey{})
	if v == nil {
		return Discard()
	}

	if l, ok := v.(*slog.Logger); ok {
		return l
	}
	return Discard()
}

// InContext returns true if there's a logger on the context.
func InContext(ctx context.Context) bool {
	v := ctx.Value(contextKey{})
	return v != nil
}

// Discard returns a new *slog.Logger that discards output.
func Discard() *slog.Logger {
	h := discardHandler{}
	return slog.New(h)
}

var _ slog.Handler = (*discardHandler)(nil)

type discardHandler struct{}

// Enabled implements slog.Handler.
func (d discardHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return false
}

// Handle implements slog.Handler.
func (d discardHandler) Handle(_ context.Context, _ slog.Record) error {
	return nil
}

// WithAttrs implements slog.Handler.
func (d discardHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return d
}

// WithGroup implements slog.Handler.
func (d discardHandler) WithGroup(_ string) slog.Handler {
	return d
}

// WarnIfError logs a warning if err is non-nil.
func WarnIfError(log *slog.Logger, msg string, err error) {
	if err == nil {
		return
	}

	if msg == "" {
		msg = "Error"
	}

	log.Warn(msg, lga.Err, err)
}

// WarnIfFuncError executes fn (if non-nil), and logs a warning
// if fn returns an error.
func WarnIfFuncError(log *slog.Logger, msg string, fn func() error) {
	if fn == nil {
		return
	}

	err := fn()
	if err == nil {
		return
	}

	if msg == "" {
		msg = "Func error"
	}

	log.Warn(msg, lga.Err, err)
}

// WarnIfCloseError executes c.Close if is non-nil, and logs a warning
// if c.Close returns an error.
func WarnIfCloseError(log *slog.Logger, msg string, c io.Closer) {
	if c == nil {
		return
	}

	err := c.Close()
	if err == nil {
		return
	}

	if msg == "" {
		msg = "Close error"
	}

	log.Warn(msg, lga.Err, err)
}

// Error logs an error if err is non-nil.
func Error(log *slog.Logger, msg string, err error, args ...any) {
	if err == nil {
		return
	}

	a := []any{lga.Err, err}
	a = append(a, args...)

	log.Error(msg, a...)
}

// Unexpected is a convenience function for logging unexpected errors
// for which there may not be any useful context message.
func Unexpected(log *slog.Logger, err error) {
	if err == nil {
		return
	}

	log.Error(lgm.Unexpected, lga.Err, err)
}
