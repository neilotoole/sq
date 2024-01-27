/*
Package tint implements a zero-dependency slog.Handler that writes tinted
(colorized) logs. The output format is inspired by the [zerolog.ConsoleWriter]
and [slog.TextHandler].

The output format can be customized using [Options], which is a drop-in
replacement for [slog.HandlerOptions].

# Customize Attributes

Options.ReplaceAttr can be used to alter or drop attributes. If set, it is
called on each non-group attribute before it is logged.
See [slog.HandlerOptions] for details.

	w := os.Stderr
	logger := slog.New(
		tint.NewHandler(w, &tint.Options{
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey && len(groups) == 0 {
					return slog.Attr{}
				}
				return a
			},
		}),
	)

# Automatically Enable Colors

Colors are enabled by default and can be disabled using the Options.NoColor
attribute. To automatically enable colors based on the terminal capabilities,
use e.g. the [go-isatty] package.

	w := os.Stderr
	logger := slog.New(
		tint.NewHandler(w, &tint.Options{
			NoColor: !isatty.IsTerminal(w.Fd()),
		}),
	)

# Windows Support

Color support on Windows can be added by using e.g. the [go-colorable] package.

	w := os.Stderr
	logger := slog.New(
		tint.NewHandler(colorable.NewColorable(w), nil),
	)

[zerolog.ConsoleWriter]: https://pkg.go.dev/github.com/rs/zerolog#ConsoleWriter
[go-isatty]: https://pkg.go.dev/github.com/mattn/go-isatty
[go-colorable]: https://pkg.go.dev/github.com/mattn/go-colorable
*/package tint

import (
	"bytes"
	"context"
	"encoding"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// ANSI modes
// See: https://gist.github.com/JBlond/2fea43a3049b38287e5e9cefc87b2124
const (
	ansiBlue             = "\033[34m"
	ansiBrightBlue       = "\033[94m"
	ansiBrightGreen      = "\033[92m"
	ansiBrightGreenBold  = "\033[1;92m"
	ansiBrightGreenFaint = "\033[92;2m"
	ansiBrightRed        = "\033[91m"
	ansiBrightRedBold    = "\033[1;91m"
	ansiBrightRedFaint   = "\033[91;2m"
	ansiBrightYellow     = "\033[93m"
	ansiFaint            = "\033[2m"
	ansiYellowBold       = "\033[1;33m"
	ansiYellow           = "\033[33m"
	ansiPurpleBold       = "\033[1;35m"

	ansiReset      = "\033[0m"
	ansiResetFaint = "\033[22m"

	ansiAttr         = "\033[36;2m"
	ansiStack        = "\033[0;35m"
	ansiStackErr     = ansiYellowBold
	ansiStackErrType = ansiBrightGreenFaint
	ansiDebug        = ansiBrightGreen
	ansiInfo         = ansiYellow
	ansiWarn         = ansiPurpleBold
	ansiError        = ansiBrightRedBold
)

const errKey = "err"

var (
	defaultLevel      = slog.LevelInfo
	defaultTimeFormat = time.StampMilli
)

// Options for a slog.Handler that writes tinted logs. A zero Options consists
// entirely of default values.
//
// Options can be used as a drop-in replacement for slog.HandlerOptions.
type Options struct {
	// Minimum level to log (Default: slog.LevelInfo)
	Level slog.Leveler

	// ReplaceAttr is called to rewrite each non-group attribute before it is logged.
	// See https://pkg.go.dev/log/slog#HandlerOptions for details.
	ReplaceAttr func(groups []string, attr slog.Attr) slog.Attr

	// Time format (Default: time.StampMilli)
	TimeFormat string

	// Enable source code location (Default: false)
	AddSource bool

	// Disable color (Default: false)
	NoColor bool
}

// NewHandler creates a slog.Handler that writes tinted logs to Writer w,
// using the default options. If opts is nil, the default options are used.
func NewHandler(w io.Writer, opts *Options) slog.Handler {
	h := &handler{
		w:          w,
		level:      defaultLevel,
		timeFormat: defaultTimeFormat,
	}
	if opts == nil {
		return h
	}

	h.addSource = opts.AddSource
	if opts.Level != nil {
		h.level = opts.Level
	}
	h.replaceAttr = opts.ReplaceAttr
	if opts.TimeFormat != "" {
		h.timeFormat = opts.TimeFormat
	}
	h.noColor = opts.NoColor
	return h
}

// handler implements a slog.Handler.
type handler struct {
	w io.Writer

	level       slog.Leveler
	replaceAttr func([]string, slog.Attr) slog.Attr
	attrsPrefix string
	groupPrefix string
	timeFormat  string
	groups      []string

	mu sync.Mutex

	addSource bool
	noColor   bool
}

func (h *handler) clone() *handler {
	return &handler{
		attrsPrefix: h.attrsPrefix,
		groupPrefix: h.groupPrefix,
		groups:      h.groups,
		w:           h.w,
		addSource:   h.addSource,
		level:       h.level,
		replaceAttr: h.replaceAttr,
		timeFormat:  h.timeFormat,
		noColor:     h.noColor,
	}
}

func (h *handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	// get a buffer from the sync pool
	buf := newBuffer()
	defer buf.Free()

	rep := h.replaceAttr

	// write time
	if !r.Time.IsZero() {
		val := r.Time.Round(0) // strip monotonic to match Attr behavior
		if rep == nil {
			h.appendTime(buf, r.Time)
			buf.WriteByte(' ')
		} else if a := rep(nil /* groups */, slog.Time(slog.TimeKey, val)); a.Key != "" {
			if a.Value.Kind() == slog.KindTime {
				h.appendTime(buf, a.Value.Time())
			} else {
				h.appendValue(buf, a.Value, false)
			}
			buf.WriteByte(' ')
		}
	}

	// write level
	if rep == nil {
		h.appendLevel(buf, r.Level)
		buf.WriteByte(' ')
	} else if a := rep(nil /* groups */, slog.Any(slog.LevelKey, r.Level)); a.Key != "" {
		h.appendValue(buf, a.Value, false)
		buf.WriteByte(' ')
	}

	// write source
	if h.addSource {
		fs := runtime.CallersFrames([]uintptr{r.PC})
		f, _ := fs.Next()
		if f.File != "" {
			src := &slog.Source{
				Function: f.Function,
				File:     f.File,
				Line:     f.Line,
			}

			if rep == nil {
				h.appendSource(buf, src)
				buf.WriteByte(' ')
			} else if a := rep(nil /* groups */, slog.Any(slog.SourceKey, src)); a.Key != "" {
				h.appendValue(buf, a.Value, false)
				buf.WriteByte(' ')
			}
		}
	}

	msgColor := ansiBrightGreen
	switch r.Level {
	case slog.LevelDebug:
		msgColor = ansiDebug
	case slog.LevelWarn:
		msgColor = ansiWarn
	case slog.LevelError:
		msgColor = ansiBrightRedBold
	case slog.LevelInfo:
		msgColor = ansiInfo
	}
	// write message
	if rep == nil {
		buf.WriteStringIf(!h.noColor, msgColor)
		buf.WriteString(r.Message)
		buf.WriteStringIf(!h.noColor, ansiReset)
		buf.WriteByte(' ')
	} else if a := rep(nil /* groups */, slog.String(slog.MessageKey, r.Message)); a.Key != "" {
		buf.WriteStringIf(!h.noColor, msgColor)
		h.appendValue(buf, a.Value, false)
		buf.WriteStringIf(!h.noColor, ansiReset)
		buf.WriteByte(' ')
	}

	// write handler attributes
	if len(h.attrsPrefix) > 0 {
		buf.WriteString(h.attrsPrefix)
	}

	const keyStack = "stack"
	var stackAttrs []slog.Attr
	var resps []*http.Response

	// write attributes
	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == keyStack {
			// Special handling for stacktraces
			stackAttrs = append(stackAttrs, attr)
			return true
		}

		if resp, ok := attr.Value.Any().(*http.Response); ok {
			// Special handling for http responses
			resps = append(resps, resp)
			return true
		}

		h.appendAttr(buf, attr, h.groupPrefix, h.groups)
		return true
	})

	h.handleHTTPResponse(buf, resps)

	if len(*buf) == 0 {
		return nil
	}
	(*buf)[len(*buf)-1] = '\n' // replace last space with newline

	h.handleStackAttrs(buf, stackAttrs)

	h.mu.Lock()
	defer h.mu.Unlock()

	_, err := h.w.Write(*buf)
	return err
}

func (h *handler) handleHTTPResponse(buf *buffer, resps []*http.Response) {
	for _, resp := range resps {
		if resp == nil {
			continue
		}
		b, _ := httputil.DumpResponse(resp, false)
		b = bytes.TrimSpace(b)
		if len(b) == 0 {
			return
		}

		buf.WriteByte('\n')
		buf.WriteStringIf(!h.noColor, ansiAttr)
		_, _ = buf.Write(b)
		buf.WriteStringIf(!h.noColor, ansiReset)
		buf.WriteByte('\n')
	}
}

func (h *handler) handleStackAttrs(buf *buffer, attrs []slog.Attr) {
	if len(attrs) == 0 {
		return
	}
	var stacks []*errz.StackTrace
	for _, attr := range attrs {
		switch v := attr.Value.Any().(type) {
		case *errz.StackTrace:
			if v != nil {
				stacks = append(stacks, v)
			}
		case []*errz.StackTrace:
			stacks = append(stacks, v...)
		}
	}

	var printed int
	for _, stack := range stacks {
		if stack == nil {
			continue
		}

		stackPrint := fmt.Sprintf("%+v", stack.Frames)
		stackPrint = strings.ReplaceAll(strings.TrimSpace(stackPrint), "\n\t", "\n  ")
		if stackPrint == "" {
			continue
		}

		if printed > 0 {
			buf.WriteString("\n")
		}

		if stack.Error != nil {
			buf.WriteStringIf(!h.noColor, ansiStackErrType)
			buf.WriteString(errz.SprintTreeTypes(stack.Error))
			buf.WriteStringIf(!h.noColor, ansiReset)
			buf.WriteByte('\n')

			buf.WriteStringIf(!h.noColor, ansiStackErr)
			buf.WriteString(stack.Error.Error())
			buf.WriteStringIf(!h.noColor, ansiReset)
			buf.WriteByte('\n')
		}
		lines := strings.Split(stackPrint, "\n")
		for _, line := range lines {
			buf.WriteStringIf(!h.noColor, ansiStack)
			buf.WriteString(line)
			buf.WriteStringIf(!h.noColor, ansiReset)
			buf.WriteByte('\n')
		}
		printed++
	}
	if printed > 0 {
		buf.WriteByte('\n')
	}
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	h2 := h.clone()

	buf := newBuffer()
	defer buf.Free()

	// write attributes to buffer
	for _, attr := range attrs {
		h.appendAttr(buf, attr, h.groupPrefix, h.groups)
	}
	h2.attrsPrefix = h.attrsPrefix + string(*buf)
	return h2
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	h2 := h.clone()
	h2.groupPrefix += name + "."
	h2.groups = append(h2.groups, name)
	return h2
}

func (h *handler) appendTime(buf *buffer, t time.Time) {
	buf.WriteStringIf(!h.noColor, ansiFaint)
	*buf = t.AppendFormat(*buf, h.timeFormat)
	buf.WriteStringIf(!h.noColor, ansiReset)
}

func (h *handler) appendLevel(buf *buffer, level slog.Level) {
	switch {
	case level < slog.LevelInfo:
		buf.WriteStringIf(!h.noColor, ansiDebug)
		buf.WriteString("DBG")
		appendLevelDelta(buf, level-slog.LevelDebug)
		buf.WriteStringIf(!h.noColor, ansiReset)
	case level < slog.LevelWarn:
		buf.WriteStringIf(!h.noColor, ansiInfo)
		buf.WriteString("INF")
		appendLevelDelta(buf, level-slog.LevelInfo)
		buf.WriteStringIf(!h.noColor, ansiReset)
	case level < slog.LevelError:
		buf.WriteStringIf(!h.noColor, ansiWarn)
		buf.WriteString("WRN")
		appendLevelDelta(buf, level-slog.LevelWarn)
		buf.WriteStringIf(!h.noColor, ansiReset)
	default:
		buf.WriteStringIf(!h.noColor, ansiError)
		buf.WriteString("ERR")
		appendLevelDelta(buf, level-slog.LevelError)
		buf.WriteStringIf(!h.noColor, ansiReset)
	}
}

func appendLevelDelta(buf *buffer, delta slog.Level) {
	if delta == 0 {
		return
	} else if delta > 0 {
		buf.WriteByte('+')
	}
	*buf = strconv.AppendInt(*buf, int64(delta), 10)
}

func (h *handler) appendSource(buf *buffer, src *slog.Source) {
	dir, file := filepath.Split(src.File)

	fn := src.Function
	parts := strings.Split(src.Function, "/")
	if len(parts) > 0 {
		fn = parts[len(parts)-1]
	}

	if fn != "" {
		buf.WriteStringIf(!h.noColor, ansiBlue)
		buf.WriteString(fn)
		buf.WriteStringIf(!h.noColor, ansiReset)
		buf.WriteByte(' ')
	}
	buf.WriteStringIf(!h.noColor, ansiFaint)
	buf.WriteString(filepath.Join(filepath.Base(dir), file))
	buf.WriteByte(':')
	buf.WriteString(strconv.Itoa(src.Line))
	buf.WriteStringIf(!h.noColor, ansiReset)
}

func (h *handler) appendAttr(buf *buffer, a slog.Attr, groupsPrefix string, groups []string) {
	if rep := h.replaceAttr; rep != nil && a.Value.Kind() != slog.KindGroup {
		a.Value = a.Value.Resolve()
		a = rep(groups, a)
	}
	a.Value = a.Value.Resolve()

	if a.Equal(slog.Attr{}) {
		return
	}

	if a.Value.Kind() == slog.KindGroup {
		if a.Key != "" {
			groupsPrefix += a.Key + "."
			groups = append(groups, a.Key)
		}
		for _, groupAttr := range a.Value.Group() {
			h.appendAttr(buf, groupAttr, groupsPrefix, groups)
		}
	} else if err, ok := a.Value.Any().(tintError); ok {
		// append tintError
		h.appendTintError(buf, err, groupsPrefix)
		buf.WriteByte(' ')
	} else {
		h.appendKey(buf, a.Key, groupsPrefix)
		buf.WriteStringIf(!h.noColor, ansiAttr)
		h.appendValue(buf, a.Value, true)
		buf.WriteStringIf(!h.noColor, ansiReset)

		buf.WriteByte(' ')
	}
}

func (h *handler) appendKey(buf *buffer, key, groups string) {
	buf.WriteStringIf(!h.noColor, ansiFaint)
	appendString(buf, groups+key, true)
	buf.WriteByte('=')
	buf.WriteStringIf(!h.noColor, ansiReset)
}

func (h *handler) appendValue(buf *buffer, v slog.Value, quote bool) {
	switch v.Kind() {
	case slog.KindString:
		appendString(buf, v.String(), quote)
	case slog.KindInt64:
		*buf = strconv.AppendInt(*buf, v.Int64(), 10)
	case slog.KindUint64:
		*buf = strconv.AppendUint(*buf, v.Uint64(), 10)
	case slog.KindFloat64:
		*buf = strconv.AppendFloat(*buf, v.Float64(), 'g', -1, 64)
	case slog.KindBool:
		*buf = strconv.AppendBool(*buf, v.Bool())
	case slog.KindDuration:
		appendString(buf, v.Duration().String(), quote)
	case slog.KindTime:
		appendString(buf, v.Time().String(), quote)
	case slog.KindAny:
		switch cv := v.Any().(type) {
		case slog.Level:
			h.appendLevel(buf, cv)
		case encoding.TextMarshaler:
			data, err := cv.MarshalText()
			if err != nil {
				break
			}
			appendString(buf, string(data), quote)
		case *slog.Source:
			h.appendSource(buf, cv)
		default:
			appendString(buf, fmt.Sprint(v.Any()), quote)
		}
	}
}

func (h *handler) appendTintError(buf *buffer, err error, groupsPrefix string) {
	buf.WriteStringIf(!h.noColor, ansiBrightRedFaint)
	appendString(buf, groupsPrefix+errKey, true)
	buf.WriteByte('=')
	buf.WriteStringIf(!h.noColor, ansiResetFaint)
	appendString(buf, err.Error(), true)
	buf.WriteStringIf(!h.noColor, ansiReset)
}

func appendString(buf *buffer, s string, quote bool) {
	if quote && needsQuoting(s) {
		*buf = strconv.AppendQuote(*buf, s)
	} else {
		buf.WriteString(s)
	}
}

func needsQuoting(s string) bool {
	if len(s) == 0 {
		return true
	}
	for _, r := range s {
		if unicode.IsSpace(r) || r == '"' || r == '=' || !unicode.IsPrint(r) {
			return true
		}
	}
	return false
}

type tintError struct{ error }

// Err returns a tinted (colorized) slog.Attr that will be written in red color
// by the [tint.Handler]. When used with any other slog.Handler, it behaves as
//
//	slog.Any("err", err)
func Err(err error) slog.Attr {
	if err != nil {
		err = tintError{err}
	}
	return slog.Any(errKey, err)
}
