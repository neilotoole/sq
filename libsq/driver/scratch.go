package driver

import (
	"context"

	"github.com/neilotoole/sq/libsq/source"
)

// ScratchSrcFunc is a function that returns a scratch source.
// The caller is responsible for invoking cleanFn.
type ScratchSrcFunc func(ctx context.Context, name string) (src *source.Source, cleanFn func() error, err error)
