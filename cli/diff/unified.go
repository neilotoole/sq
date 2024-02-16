package diff

// Reference:
// - https://github.com/aymanbagabas/go-udiff
// - https://www.gnu.org/software/diffutils/manual/html_node/Hunks.html
// - https://www.cloudbees.com/blog/git-diff-a-complete-comparison-tutorial-for-git

import (
	"context"
	"github.com/neilotoole/sq/cli/diff/internal/go-udiff"
	"github.com/neilotoole/sq/cli/diff/internal/go-udiff/myers"
	"github.com/neilotoole/sq/libsq/core/errz"
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
			err = errz.Err(ctx.Err())
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
		return "", errz.Err(ctx.Err())
	case <-done:
	}

	return unified, err
}
