package driver

import (
	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/libsq/source"
)

// ScratchSrcFunc is a function that returns a scratch source.
// The caller is responsible for invoking cleanFn.
type ScratchSrcFunc func(log *slog.Logger, name string) (src *source.Source, cleanFn func() error, err error)
