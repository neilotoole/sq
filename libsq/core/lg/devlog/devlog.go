// Package devlog contains a custom slog.Handler for
// developer-friendly log output.
package devlog

import (
	"io"
	"log/slog"

	"github.com/neilotoole/sq/libsq/core/lg/devlog/tint"
)

const shortTimeFormat = "15:04:05.000000"

// NewHandler returns a developer-friendly slog.Handler that
// logs to w.
func NewHandler(w io.Writer, lvl slog.Leveler) slog.Handler {
	h := tint.NewHandler(w, &tint.Options{
		Level:      lvl,
		TimeFormat: shortTimeFormat,
		AddSource:  true,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			switch a.Key {
			// case "pid":
			//	return slog.Attr{}
			case "error":
				a.Key = "err"
				return a
			default:
				return a
			}
		},
	})
	return h
}
