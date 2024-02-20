package diffdoc

import (
	"context"

	udiff "github.com/neilotoole/sq/libsq/core/diffdoc/internal/go-udiff"
	"github.com/neilotoole/sq/libsq/core/diffdoc/internal/go-udiff/myers"
)

// ComputeUnified encapsulates computing a unified diff.
func ComputeUnified(ctx context.Context, oldLabel, newLabel string, lines int,
	before, after string,
) (string, error) {
	var (
		unified string
		err     error
		done    = make(chan struct{})
	)

	// We compute the diff on a goroutine because the underlying diff
	// library functions aren't context-aware.
	go func() {
		defer close(done)

		edits := myers.ComputeEdits(before, after)
		// After edits are computed, if the context is done,
		// there's no point continuing.
		select {
		case <-ctx.Done():
			err = context.Cause(ctx)
			return
		default:
		}

		unified, err = udiff.ToUnified(
			oldLabel,
			newLabel,
			before,
			edits,
			lines,
		)
	}()

	select {
	case <-ctx.Done():
		return "", context.Cause(ctx)
	case <-done:
	}

	return unified, err
}
