package tint_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lmittmann/tint"
)

var faketime = time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC)

func Example() {
	slog.SetDefault(slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelDebug,
		TimeFormat: time.Kitchen,
	})))

	slog.Info("Starting server", "addr", ":8080", "env", "production")
	slog.Debug("Connected to DB", "db", "myapp", "host", "localhost:5432")
	slog.Warn("Slow request", "method", "GET", "path", "/users", "duration", 497*time.Millisecond)
	slog.Error("DB connection lost", tint.Err(errors.New("connection reset")), "db", "myapp")
	// Output:
}

// Run test with "faketime" tag:
//
//	TZ="" go test -tags=faketime
func TestHandler(t *testing.T) {
	if !faketime.Equal(time.Now()) {
		t.Skip(`skipping test; run with "-tags=faketime"`)
	}

	tests := []struct {
		Opts *tint.Options
		F    func(l *slog.Logger)
		Want string
	}{
		{
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 INF test key=val`,
		},
		{
			F: func(l *slog.Logger) {
				l.Error("test", tint.Err(errors.New("fail")))
			},
			Want: `Nov 10 23:00:00.000 ERR test err=fail`,
		},
		{
			F: func(l *slog.Logger) {
				l.Info("test", slog.Group("group", slog.String("key", "val"), tint.Err(errors.New("fail"))))
			},
			Want: `Nov 10 23:00:00.000 INF test group.key=val group.err=fail`,
		},
		{
			F: func(l *slog.Logger) {
				l.WithGroup("group").Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 INF test group.key=val`,
		},
		{
			F: func(l *slog.Logger) {
				l.With("key", "val").Info("test", "key2", "val2")
			},
			Want: `Nov 10 23:00:00.000 INF test key=val key2=val2`,
		},
		{
			F: func(l *slog.Logger) {
				l.Info("test", "k e y", "v a l")
			},
			Want: `Nov 10 23:00:00.000 INF test "k e y"="v a l"`,
		},
		{
			F: func(l *slog.Logger) {
				l.WithGroup("g r o u p").Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 INF test "g r o u p.key"=val`,
		},
		{
			F: func(l *slog.Logger) {
				l.Info("test", "slice", []string{"a", "b", "c"}, "map", map[string]int{"a": 1, "b": 2, "c": 3})
			},
			Want: `Nov 10 23:00:00.000 INF test slice="[a b c]" map="map[a:1 b:2 c:3]"`,
		},
		{
			Opts: &tint.Options{
				AddSource: true,
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 INF tint/handler_test.go:100 test key=val`,
		},
		{
			Opts: &tint.Options{
				TimeFormat: time.Kitchen,
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `11:00PM INF test key=val`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: drop(slog.TimeKey),
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `INF test key=val`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: drop(slog.LevelKey),
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 test key=val`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: drop(slog.MessageKey),
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 INF key=val`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: drop(slog.TimeKey, slog.LevelKey, slog.MessageKey),
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `key=val`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: drop("key"),
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 INF test`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: drop("key"),
			},
			F: func(l *slog.Logger) {
				l.WithGroup("group").Info("test", "key", "val", "key2", "val2")
			},
			Want: `Nov 10 23:00:00.000 INF test group.key=val group.key2=val2`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == "key" && len(groups) == 1 && groups[0] == "group" {
						return slog.Attr{}
					}
					return a
				},
			},
			F: func(l *slog.Logger) {
				l.WithGroup("group").Info("test", "key", "val", "key2", "val2")
			},
			Want: `Nov 10 23:00:00.000 INF test group.key2=val2`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: replace(slog.IntValue(42), slog.TimeKey),
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `42 INF test key=val`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: replace(slog.StringValue("INFO"), slog.LevelKey),
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 INFO test key=val`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: replace(slog.IntValue(42), slog.MessageKey),
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: `Nov 10 23:00:00.000 INF 42 key=val`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: replace(slog.IntValue(42), "key"),
			},
			F: func(l *slog.Logger) {
				l.With("key", "val").Info("test", "key2", "val2")
			},
			Want: `Nov 10 23:00:00.000 INF test key=42 key2=val2`,
		},
		{
			Opts: &tint.Options{
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					return slog.Attr{}
				},
			},
			F: func(l *slog.Logger) {
				l.Info("test", "key", "val")
			},
			Want: ``,
		},
		{
			F: func(l *slog.Logger) {
				l.Info("test", "key", "")
			},
			Want: `Nov 10 23:00:00.000 INF test key=""`,
		},
		{
			F: func(l *slog.Logger) {
				l.Info("test", "", "val")
			},
			Want: `Nov 10 23:00:00.000 INF test ""=val`,
		},
		{
			F: func(l *slog.Logger) {
				l.Info("test", "", "")
			},
			Want: `Nov 10 23:00:00.000 INF test ""=""`,
		},

		{ // https://github.com/lmittmann/tint/issues/8
			F: func(l *slog.Logger) {
				l.Log(context.TODO(), slog.LevelInfo+1, "test")
			},
			Want: `Nov 10 23:00:00.000 INF+1 test`,
		},
		{
			Opts: &tint.Options{
				Level: slog.LevelDebug - 1,
			},
			F: func(l *slog.Logger) {
				l.Log(context.TODO(), slog.LevelDebug-1, "test")
			},
			Want: `Nov 10 23:00:00.000 DBG-1 test`,
		},
		{ // https://github.com/lmittmann/tint/issues/12
			F: func(l *slog.Logger) {
				l.Error("test", slog.Any("error", errors.New("fail")))
			},
			Want: `Nov 10 23:00:00.000 ERR test error=fail`,
		},
		{ // https://github.com/lmittmann/tint/issues/15
			F: func(l *slog.Logger) {
				l.Error("test", tint.Err(nil))
			},
			Want: `Nov 10 23:00:00.000 ERR test err=<nil>`,
		},
		{ // https://github.com/lmittmann/tint/pull/26
			Opts: &tint.Options{
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == slog.TimeKey && len(groups) == 0 {
						return slog.Time(slog.TimeKey, a.Value.Time().Add(24*time.Hour))
					}
					return a
				},
			},
			F: func(l *slog.Logger) {
				l.Error("test")
			},
			Want: `Nov 11 23:00:00.000 ERR test`,
		},
		{ // https://github.com/lmittmann/tint/pull/27
			F: func(l *slog.Logger) {
				l.Info("test", "a", "b", slog.Group("", slog.String("c", "d")), "e", "f")
			},
			Want: `Nov 10 23:00:00.000 INF test a=b c=d e=f`,
		},
		{ // https://github.com/lmittmann/tint/pull/30
			// drop built-in attributes in a grouped log
			Opts: &tint.Options{
				ReplaceAttr: drop(slog.TimeKey, slog.LevelKey, slog.MessageKey, slog.SourceKey),
				AddSource:   true,
			},
			F: func(l *slog.Logger) {
				l.WithGroup("group").Info("test", "key", "val")
			},
			Want: `group.key=val`,
		},
		{ // https://github.com/lmittmann/tint/issues/36
			Opts: &tint.Options{
				ReplaceAttr: func(g []string, a slog.Attr) slog.Attr {
					if len(g) == 0 && a.Key == slog.LevelKey {
						_ = a.Value.Any().(slog.Level)
					}
					return a
				},
			},
			F: func(l *slog.Logger) {
				l.Info("test")
			},
			Want: `Nov 10 23:00:00.000 INF test`,
		},
		{ // https://github.com/lmittmann/tint/issues/37
			Opts: &tint.Options{
				AddSource: true,
				ReplaceAttr: func(g []string, a slog.Attr) slog.Attr {
					return a
				},
			},
			F: func(l *slog.Logger) {
				l.Info("test")
			},
			Want: `Nov 10 23:00:00.000 INF tint/handler_test.go:327 test`,
		},
		{ // https://github.com/lmittmann/tint/issues/44
			F: func(l *slog.Logger) {
				l = l.WithGroup("group")
				l.Error("test", tint.Err(errTest))
			},
			Want: `Nov 10 23:00:00.000 ERR test group.err=fail`,
		},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var buf bytes.Buffer
			if test.Opts == nil {
				test.Opts = &tint.Options{}
			}
			test.Opts.NoColor = true
			l := slog.New(tint.NewHandler(&buf, test.Opts))
			test.F(l)

			got := strings.TrimRight(buf.String(), "\n")
			if test.Want != got {
				t.Fatalf("(-want +got)\n- %s\n+ %s", test.Want, got)
			}
		})
	}
}

// drop returns a ReplaceAttr that drops the given keys.
func drop(keys ...string) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) > 0 {
			return a
		}

		for _, key := range keys {
			if a.Key == key {
				a = slog.Attr{}
			}
		}
		return a
	}
}

func replace(new slog.Value, keys ...string) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if len(groups) > 0 {
			return a
		}

		for _, key := range keys {
			if a.Key == key {
				a.Value = new
			}
		}
		return a
	}
}

func TestReplaceAttr(t *testing.T) {
	tests := [][]any{
		{},
		{"key", "val"},
		{"key", "val", slog.Group("group", "key2", "val2")},
		{"key", "val", slog.Group("group", "key2", "val2", slog.Group("group2", "key3", "val3"))},
	}

	type replaceAttrParams struct {
		Groups []string
		Attr   slog.Attr
	}

	replaceAttrRecorder := func(record *[]replaceAttrParams) func([]string, slog.Attr) slog.Attr {
		return func(groups []string, a slog.Attr) slog.Attr {
			*record = append(*record, replaceAttrParams{groups, a})
			return a
		}
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			slogRecord := make([]replaceAttrParams, 0)
			slogLogger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
				ReplaceAttr: replaceAttrRecorder(&slogRecord),
			}))
			slogLogger.Log(context.TODO(), slog.LevelInfo, "", test...)

			tintRecord := make([]replaceAttrParams, 0)
			tintLogger := slog.New(tint.NewHandler(io.Discard, &tint.Options{
				ReplaceAttr: replaceAttrRecorder(&tintRecord),
			}))
			tintLogger.Log(context.TODO(), slog.LevelInfo, "", test...)

			if !slices.EqualFunc(slogRecord, tintRecord, func(a, b replaceAttrParams) bool {
				return slices.Equal(a.Groups, b.Groups) && a.Attr.Equal(b.Attr)
			}) {
				t.Fatalf("(-want +got)\n- %v\n+ %v", slogRecord, tintRecord)
			}
		})
	}
}

// See https://github.com/golang/exp/blob/master/slog/benchmarks/benchmarks_test.go#L25
//
// Run e.g.:
//
//	go test -bench=. -count=10 | benchstat -col /h /dev/stdin
func BenchmarkLogAttrs(b *testing.B) {
	handler := []struct {
		Name string
		H    slog.Handler
	}{
		{"tint", tint.NewHandler(io.Discard, nil)},
		{"text", slog.NewTextHandler(io.Discard, nil)},
		{"json", slog.NewJSONHandler(io.Discard, nil)},
		{"discard", new(discarder)},
	}

	benchmarks := []struct {
		Name string
		F    func(*slog.Logger)
	}{
		{
			"5 args",
			func(logger *slog.Logger) {
				logger.LogAttrs(context.TODO(), slog.LevelInfo, testMessage,
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
				)
			},
		},
		{
			"5 args custom level",
			func(logger *slog.Logger) {
				logger.LogAttrs(context.TODO(), slog.LevelInfo+1, testMessage,
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
				)
			},
		},
		{
			"10 args",
			func(logger *slog.Logger) {
				logger.LogAttrs(context.TODO(), slog.LevelInfo, testMessage,
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
				)
			},
		},
		{
			"40 args",
			func(logger *slog.Logger) {
				logger.LogAttrs(context.TODO(), slog.LevelInfo, testMessage,
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
					slog.String("string", testString),
					slog.Int("status", testInt),
					slog.Duration("duration", testDuration),
					slog.Time("time", testTime),
					slog.Any("error", errTest),
				)
			},
		},
	}

	for _, h := range handler {
		b.Run("h="+h.Name, func(b *testing.B) {
			for _, bench := range benchmarks {
				b.Run(bench.Name, func(b *testing.B) {
					b.ReportAllocs()
					logger := slog.New(h.H)
					for i := 0; i < b.N; i++ {
						bench.F(logger)
					}
				})
			}
		})
	}
}

// discarder is a slog.Handler that discards all records.
type discarder struct{}

func (*discarder) Enabled(context.Context, slog.Level) bool   { return true }
func (*discarder) Handle(context.Context, slog.Record) error  { return nil }
func (d *discarder) WithAttrs(attrs []slog.Attr) slog.Handler { return d }
func (d *discarder) WithGroup(name string) slog.Handler       { return d }

var (
	testMessage  = "Test logging, but use a somewhat realistic message length."
	testTime     = time.Date(2022, time.May, 1, 0, 0, 0, 0, time.UTC)
	testString   = "7e3b3b2aaeff56a7108fe11e154200dd/7819479873059528190"
	testInt      = 32768
	testDuration = 23 * time.Second
	errTest      = errors.New("fail")
)
