package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/source"
)

// maybeExpandCollection returns coll unchanged when --expand is not set
// on cmd. Otherwise it returns a deep clone whose source Locations have
// each been passed through ru.SecretRegistry.Expand, with lenient
// fallback: any per-source resolver error is swallowed and the
// original Location is preserved verbatim. ctx is used for the
// resolver call; context.Canceled / context.DeadlineExceeded propagate
// as errors (a partial expansion under user-driven cancellation is not
// a "this source's keyring is offline" situation).
//
// Display commands (sq src, sq ls, sq inspect, sq add echo, sq mv
// echo, sq ping -v) call this at the start of execution and pass the
// returned collection to the writer. The writer's existing redact
// step runs on whatever Location it sees, so the matrix is:
//
//	raw -> [expand?] -> [redact?] -> displayed
//
// See also: maybeExpandSource for the single-source variant.
func maybeExpandCollection(ctx context.Context, ru *run.Run, cmd *cobra.Command,
	coll *source.Collection,
) (*source.Collection, error) {
	if !cmdFlagIsSetTrue(cmd, flag.Expand) || coll == nil {
		return coll, nil
	}

	clone := coll.Clone()
	for _, src := range clone.Sources() {
		resolved, err := ru.SecretRegistry.Expand(ctx, src.Location)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			// Lenient: leave the placeholder verbatim so the user can
			// still read "what would have resolved here". No stderr
			// noise: the verbatim placeholder is the signal.
			continue
		}
		src.Location = resolved
	}
	return clone, nil
}

// maybeExpandSource is the single-source variant of
// maybeExpandCollection. Same semantics: --expand unset returns input
// verbatim; --expand set returns a cloned source with Expand applied,
// lenient on resolver error, propagates context cancellation.
func maybeExpandSource(ctx context.Context, ru *run.Run, cmd *cobra.Command,
	src *source.Source,
) (*source.Source, error) {
	if !cmdFlagIsSetTrue(cmd, flag.Expand) || src == nil {
		return src, nil
	}

	clone := src.Clone()
	resolved, err := ru.SecretRegistry.Expand(ctx, clone.Location)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return clone, nil
	}
	clone.Location = resolved
	return clone, nil
}
