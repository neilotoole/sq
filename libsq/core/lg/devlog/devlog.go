package devlog

import (
	"github.com/neilotoole/sq/libsq/core/lg/devlog/tint"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
)

const shortTimeFormat = `15:04:05.000000`

// New returns a developer-friendly logger that
// logs to w.
func New(w io.Writer, lvl slog.Leveler) *slog.Logger {
	h := NewHandler(w, lvl)
	return slog.New(h)
}

func NewHandler(w io.Writer, lvl slog.Leveler) slog.Handler {
	h := tint.NewHandler(w, &tint.Options{
		Level:      lvl,
		TimeFormat: shortTimeFormat,
		AddSource:  true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case "pid":
				return slog.Attr{}
			default:
				return a
			}
		},
	})
	return h
}

// replaceSourceShort prints a dev-friendly "source" field.
func replaceSourceShort(_ []string, a slog.Attr) slog.Attr {
	if src, ok := a.Value.Any().(*slog.Source); ok {
		s := filepath.Join(filepath.Base(filepath.Dir(src.File)), filepath.Base(src.File))
		s += ":" + strconv.Itoa(src.Line)

		fn := src.Function
		parts := strings.Split(src.Function, "/")
		if len(parts) > 0 {
			fn = parts[len(parts)-1]
		}

		s += ":" + fn
		//a.Key = "src"
		a.Value = slog.StringValue(s)
	}
	return a
}
