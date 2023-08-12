// Package slogbuf implements a Buffer that stores log records
// that can later be replayed on a slog.Handler. This is particularly
// useful for bootstrap.
//
// Example: let's say log configuration is stored in an external config source,
// but the config load mechanism itself wants a logger. A slogbuf "boot logger"
// can be passed to the config load mechanism. On successful boot, the "real"
// logger can be created, and slogbuf's records can be replayed on that logger.
// If boot fails, slogbuf's records can be replayed to stderr.
package slogbuf

import (
	"context"
	"log/slog"
	"sync"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// New returns a logger, and a buffer that can be used for replay.
func New() (*slog.Logger, *Buffer) {
	h := &handler{
		buf: &Buffer{},
	}

	return slog.New(h), h.buf
}

// Buffer stores slog records that can be replayed via Buffer.Flush.
type Buffer struct {
	mu      sync.Mutex
	entries []entry
}

type entry struct {
	handler *handler
	record  slog.Record
}

func (b *Buffer) append(h *handler, record slog.Record) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries = append(b.entries, entry{
		handler: h,
		record:  record,
	})
}

// Flush replays the buffer on dest. If an error occurs writing the
// log records to dest, Flush returns immediately (and does not write
// any remaining records). The buffer drains, even if an error occurs.
func (b *Buffer) Flush(ctx context.Context, dest slog.Handler) error {
	if dest == nil {
		return errz.New("flush log buffer: dest is nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	defer func() { b.entries = nil }()

	for i := range b.entries {
		d, h, rec := dest, b.entries[i].handler, b.entries[i].record

		if !d.Enabled(ctx, rec.Level) {
			continue
		}

		d = d.WithAttrs(h.attrs)
		for _, g := range h.groups {
			d = d.WithGroup(g)
		}

		if err := d.Handle(ctx, rec); err != nil {
			return err
		}
	}

	return nil
}

var _ slog.Handler = (*handler)(nil)

// handler implements slog.Handler.
type handler struct {
	buf    *Buffer
	attrs  []slog.Attr
	groups []string
}

// Enabled implements slog.Handler.
func (h *handler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

// WithAttrs implements slog.Handler.
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := &handler{buf: h.buf}

	if h.groups != nil {
		h2.groups = make([]string, len(h.groups))
		copy(h2.groups, h.groups)
	}

	h2.attrs = make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(h2.attrs, h.attrs)
	h2.attrs = append(h2.attrs, attrs...)

	return h2
}

// WithGroup implements slog.Handler.
func (h *handler) WithGroup(name string) slog.Handler {
	h2 := &handler{buf: h.buf}

	if h.attrs != nil {
		h2.attrs = make([]slog.Attr, len(h.attrs))
		copy(h2.attrs, h.attrs)
	}

	h2.groups = make([]string, len(h.groups)+1)
	copy(h2.groups, h.groups)
	h2.groups[len(h2.groups)-1] = name

	return h2
}

// Handle implements slog.Handler.
func (h *handler) Handle(_ context.Context, record slog.Record) error {
	h.buf.append(h, record)
	return nil
}
