// Package lgt provides a mechanism for getting a *slog.Logger
// that outputs to testing.T. See [lgt.New].
package lgt

import (
	"io"
	"log/slog"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/libsq/core/lg/devlog"
)

func init() { //nolint:gochecknoinits
	slogt.Default = slogt.Factory(func(w io.Writer) slog.Handler {
		return devlog.NewHandler(w, slog.LevelDebug)
	})
}

// New delegates to [slogt.New].
var New = slogt.New
