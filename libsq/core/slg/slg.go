// Package slg implements the slog.NewContext and slog.FromContext funcs
// that have recently been zapped from the slog proposal. I think you
// had it right the first time, Go team. Hopefully this package is short-lived
// and those funcs are put back.
package slg

import (
	"context"
	"io"

	"golang.org/x/exp/slog"
)

type contextKey struct{}

// NewContext returns a context that contains the given Logger.
// Use FromContext to retrieve the Logger.
func NewContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext returns the Logger stored in ctx by NewContext, or the default
// Logger if there is none.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return l
	}
	return Discard()
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

func WarnIfError(l *slog.Logger, err error) {
	if err == nil {
		return
	}

	l.Warn(err.Error())
}

func WarnIfFuncError(l *slog.Logger, fn func() error) {
	if fn == nil {
		return
	}

	err := fn()
	if err == nil {
		return
	}

	l.Warn(err.Error())
}

func WarnIfCloseError(l *slog.Logger, c io.Closer) {
	if c == nil {
		return
	}

	err := c.Close()
	if err == nil {
		return
	}

	l.Warn(err.Error())
}
