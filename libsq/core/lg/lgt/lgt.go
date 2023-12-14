// Package lgt provides a mechanism for getting a *slog.Logger
// that outputs to testing.T.
package lgt

import (
	"github.com/neilotoole/slogt"
	"github.com/neilotoole/sq/libsq/core/lg/devlog"
	"io"
	"log/slog"
)

func init() { //nolint:gochecknoinits
	slogt.Default = slogt.Factory(func(w io.Writer) slog.Handler {
		return devlog.NewHandler(w, slog.LevelDebug)
	})
}

// New delegates to slogt.New.
var New = slogt.New
