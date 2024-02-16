// Package lg contains utility functions for working with slog.
// It implements the slog.NewContext and slog.FromContext funcs
// that have recently been zapped from the slog proposal. I think you
// had it right the first time, Go team. Hopefully this package is short-lived
// and those funcs are put back.
package lg

import (
	"context"
	"io"
	"log/slog"
	"runtime"
	"time"

	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
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
	if ctx == nil {
		return Discard()
	}

	v := ctx.Value(contextKey{})
	if v == nil {
		return Discard()
	}

	if l, ok := v.(*slog.Logger); ok {
		return l
	}
	return Discard()
}

// Contexter is an interface for types that can return a context.Context.
type Contexter interface {
	Context() context.Context
}

// From returns the Logger stored in c's context by NewContext, or the Discard
// logger if there is none. It is a companion to FromContext.
func From(c Contexter) *slog.Logger {
	if c == nil {
		return Discard()
	}
	ctx := c.Context()
	if ctx == nil {
		return Discard()
	}

	return FromContext(ctx)
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

// IsDiscard returns true if log was returned by [lg.Discard].
func IsDiscard(log *slog.Logger) bool {
	if log == nil {
		return false
	}

	h := log.Handler()
	if h == nil {
		return false
	}

	if _, ok := h.(discardHandler); ok {
		return true
	}

	return false
}

// WarnIfError logs a warning if err is non-nil.
func WarnIfError(log *slog.Logger, msg string, err error) {
	if err == nil {
		return
	}

	if msg == "" {
		msg = "Error"
	}

	Depth(log, slog.LevelWarn, 1, msg, lga.Err, err)
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

	Depth(log, slog.LevelWarn, 1, msg, lga.Err, err)
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

	Depth(log, slog.LevelWarn, 1, msg, lga.Err, err)
}

// Unexpected is a convenience function for logging unexpected errors
// for which there may not be any useful context message.
func Unexpected(log *slog.Logger, err error) {
	if err == nil {
		return
	}

	Depth(log, slog.LevelError, 1, lgm.Unexpected, lga.Err, err)
}

// Depth logs a message with the given call (pc skip) depth.
// This is useful for logging inside a helper function.
func Depth(log *slog.Logger, level slog.Level, depth int, msg string, args ...any) {
	h := log.Handler()
	ctx := context.Background()

	if !h.Enabled(ctx, level) {
		return
	}

	var pc uintptr
	var pcs [1]uintptr
	runtime.Callers(2+depth, pcs[:])
	pc = pcs[0]

	r := slog.NewRecord(time.Now(), level, msg, pc)
	r.Add(args...)
	_ = h.Handle(ctx, r)
}
