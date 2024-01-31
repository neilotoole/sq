// Package lgt provides a mechanism for getting a *slog.Logger
// that outputs to testing.T. See lgt.New.
package lgt

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/neilotoole/slogt"

	"github.com/neilotoole/sq/libsq/core/lg/devlog"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

func init() { //nolint:gochecknoinits
	slogt.Default = slogt.Factory(func(w io.Writer) slog.Handler {
		return devlog.NewHandler(w, slog.LevelDebug)
	})
}

// New delegates to slogt.New.
func New(tb testing.TB) *slog.Logger { //nolint:thelper
	return slogt.New(tb).With(lga.Pid, os.Getpid())
}
