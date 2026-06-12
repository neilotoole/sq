package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/secret"
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
// Called by every command that prints a source location: sq src,
// sq ls, sq inspect, sq add and sq mv (post-action echo), and
// sq ping in JSON/YAML output. Each handler invokes this once
// before passing data to the writer. The writer's existing redact
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
		// Validate placeholder syntax upfront. Parse errors are user config
		// bugs (e.g. "${env}" missing the colon) and must surface so the
		// user can fix them; swallowing them silently would hide a config
		// break behind the lenient-resolver fallback below. cmd_add does the
		// same dance for added sources.
		if _, parseErr := secret.ExtractRefs(src.Location); parseErr != nil {
			return nil, errz.Wrapf(parseErr, "expand %s", src.Handle)
		}
		resolved, err := ru.SecretRegistry.Expand(ctx, src.Location)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, errz.Err(err)
			}
			// Lenient resolver failure: keep the placeholder verbatim and
			// log at debug level. The verbatim placeholder may itself be
			// hidden downstream (e.g. a placeholder inside a URL password
			// slot is masked by the redact filter when --reveal is off),
			// so the debug log is the reliable signal for operators
			// running with SQ_LOG=debug.
			lg.FromContext(ctx).Debug("expand: leaving placeholder verbatim",
				lga.Src, src.Handle,
				lga.Err, err)
			continue
		}
		src.Location = resolved
		// Resolved bytes are literal, not a template: mark the source
		// so a downstream ResolveSourceSecrets pass is a no-op. The
		// lenient-failure branch above deliberately does NOT mark, as
		// the placeholder is kept verbatim there.
		src.SecretsResolved = true
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

	// Validate placeholder syntax upfront. Parse errors are user config
	// bugs (e.g. "${env}" missing the colon) and must surface so the
	// user can fix them; swallowing them silently would hide a config
	// break behind the lenient-resolver fallback below.
	if _, parseErr := secret.ExtractRefs(src.Location); parseErr != nil {
		return nil, errz.Wrapf(parseErr, "expand %s", src.Handle)
	}
	clone := src.Clone()
	resolved, err := ru.SecretRegistry.Expand(ctx, clone.Location)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, errz.Err(err)
		}
		// Lenient resolver failure: keep the placeholder verbatim and
		// log at debug level. The verbatim value is the user-visible
		// signal in default output; the debug log is for operators
		// running with SQ_LOG=debug.
		lg.FromContext(ctx).Debug("expand: leaving placeholder verbatim",
			lga.Src, src.Handle,
			lga.Err, err)
		return clone, nil
	}
	clone.Location = resolved
	// Resolved bytes are literal, not a template: mark the source so a
	// downstream ResolveSourceSecrets pass is a no-op. The lenient
	// branch above does not mark, as the placeholder is kept verbatim.
	clone.SecretsResolved = true
	return clone, nil
}
