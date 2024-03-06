package slogbuf_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/neilotoole/sq/libsq/core/lg/slogbuf"
)

var (
	aliceAttr  = slog.String("user", "alice")
	moduleAttr = slog.String("module", "bootstrap")
	group1     = "group1"
	group2     = "group2"
)

func TestBuffer(t *testing.T) {
	const want = `{"level":"ERROR","msg":"hello error","user":"alice"}
`
	out := &bytes.Buffer{}
	dest := newTimelessJSONHandler(out)
	log, buf := slogbuf.New()

	log.Error("hello error", aliceAttr)

	if err := buf.Flush(context.Background(), dest); err != nil {
		t.Error(err)
	}

	got := out.String()
	requireEqual(t, want, got)
}

func TestBuffer_With(t *testing.T) {
	const want = `{"level":"ERROR","msg":"hello error no module","user":"alice"}
{"level":"ERROR","msg":"hello error with module","module":"bootstrap","user":"alice"}
`
	out := &bytes.Buffer{}
	dest := newTimelessJSONHandler(out)

	log, buf := slogbuf.New()

	log.Error("hello error no module", aliceAttr)

	log = log.With(moduleAttr)
	log.Error("hello error with module", aliceAttr)

	if err := buf.Flush(context.Background(), dest); err != nil {
		t.Error(err)
	}

	got := out.String()
	requireEqual(t, want, got)
}

//nolint:lll
func TestBuffer_WithGroup(t *testing.T) {
	const want = `{"level":"ERROR","msg":"hello error with module","module":"bootstrap","user":"alice"}
{"level":"ERROR","msg":"hello error with module, group1","module":"bootstrap","group1":{"user":"alice"}}
{"level":"ERROR","msg":"hello error with module, group1, group2","module":"bootstrap","group1":{"group2":{"user":"alice"}}}
`

	out := &bytes.Buffer{}
	dest := newTimelessJSONHandler(out)

	log, buf := slogbuf.New()

	log = log.With(moduleAttr)
	log.Error("hello error with module", aliceAttr)
	log = log.WithGroup(group1)
	log.Error("hello error with module, group1", aliceAttr)
	log = log.WithGroup(group2)
	log.Error("hello error with module, group1, group2", aliceAttr)

	if err := buf.Flush(context.Background(), dest); err != nil {
		t.Error(err)
	}

	got := out.String()
	requireEqual(t, want, got)
}

// TestSlogBaseline tests the baseline behavior of slog. The test
// exists because slog is still experimental at the time of writing,
// and, theoretically, slog's output could change.
func TestSlogBaseline(t *testing.T) {
	const want = `{"level":"ERROR","msg":"hello error no module","user":"alice"}
{"level":"ERROR","msg":"hello error with module","module":"bootstrap","user":"alice"}
{"level":"ERROR","msg":"hello group 1","module":"bootstrap","group1":{"user":"alice"}}
{"level":"ERROR","msg":"hello group 2","module":"bootstrap","group1":{"group2":{"user":"alice"}}}
`
	out := &bytes.Buffer{}
	dest := newTimelessJSONHandler(out)

	log := slog.New(dest)

	log.Error("hello error no module", aliceAttr)
	log = log.With(moduleAttr)
	log.Error("hello error with module", aliceAttr)

	log = log.WithGroup(group1)
	log.Error("hello group 1", aliceAttr)
	log = log.WithGroup(group2)
	log.Error("hello group 2", aliceAttr)

	got := out.String()
	requireEqual(t, want, got)
}

// newTimelessJSONHandler returns a *slog.JSONHandler that doesn't print
// the time attribute. We do this to make it easier to compare test
// output.
func newTimelessJSONHandler(w io.Writer) *slog.JSONHandler {
	h := &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelDebug,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				return slog.Attr{}
			}

			return a
		},
	}

	return slog.NewJSONHandler(w, h)
}

func requireEqual(t *testing.T, want, got string) {
	t.Helper()
	if got != want {
		t.Errorf("Output does not match.\nWANT:\n%s\nGOT:\n%s", want, got)
	}
}
