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

// This file implements the --expand flag's location expansion. The
// expansion is applied centrally in the writer layer: newWriters wraps
// the writers that print source locations (Source, Ping, Metadata) in
// the expand decorators from expand_writer.go, so any command that
// prints a location honors --expand without per-command code. The
// matrix is:
//
//	raw -> [expand?] -> [redact?] -> displayed
//
// where the redact step is the writer impls' pr.Redact handling.

// expandLocation returns loc with ${scheme:path} placeholders expanded
// via ru.SecretRegistry, with lenient fallback: a resolver error is
// swallowed (logged at debug level) and loc is returned verbatim with
// resolved=false. Context cancellation propagates as an error (a
// partial expansion under user-driven cancellation is not a "this
// source's keyring is offline" situation), as does a placeholder parse
// error: parse errors are user config bugs (e.g. "${env}" missing the
// colon) and must surface so the user can fix them; swallowing them
// silently would hide a config break behind the lenient-resolver
// fallback. The handle arg is used only for error and log context.
func expandLocation(ctx context.Context, ru *run.Run, handle, loc string,
) (expanded string, resolved bool, err error) {
	// ExtractRefs is what classifies a parse error (strict) from a
	// resolver error (lenient): Registry.Expand below collapses both
	// into an undifferentiated error, so the placeholder syntax is
	// parsed here first to surface parse errors before the lenient
	// resolver branch can swallow them. The re-parse inside Expand is
	// the accepted cost of that classification; do not drop this call.
	if _, parseErr := secret.ExtractRefs(loc); parseErr != nil {
		return "", false, errz.Wrapf(parseErr, "expand %s", handle)
	}

	expanded, expandErr := ru.SecretRegistry.Expand(ctx, loc)
	if expandErr != nil {
		if errors.Is(expandErr, context.Canceled) || errors.Is(expandErr, context.DeadlineExceeded) {
			return "", false, errz.Err(expandErr)
		}
		// Lenient resolver failure: keep the placeholder verbatim and
		// log at debug level. The verbatim placeholder may itself be
		// hidden downstream (e.g. a placeholder inside a URL password
		// slot is masked by the redact filter when --reveal is off),
		// so the debug log is the reliable signal for operators
		// running with SQ_LOG=debug.
		lg.FromContext(ctx).Debug("expand: leaving placeholder verbatim",
			lga.Src, handle,
			lga.Err, expandErr)
		return loc, false, nil
	}
	return expanded, true, nil
}

// expandSourceLocation expands src.Location in place via expandLocation,
// unless src is already a resolved literal (Source.SecretsResolved), in
// which case it is left unchanged: re-resolving a literal would unescape
// '$$' a second time, corrupting locations (e.g. those escaped by the
// v0.54.0 config upgrade, or a resolved secret value containing '$$').
// It does NOT check the --expand flag; callers gate on that. This is the
// single skip+expand site shared by maybeExpandSource and
// maybeExpandCollection (and, via the former, maybeExpandGroup).
func expandSourceLocation(ctx context.Context, ru *run.Run, src *source.Source) error {
	if src == nil || src.SecretsResolved {
		return nil
	}
	loc, resolved, err := expandLocation(ctx, ru, src.Handle, src.Location)
	if err != nil {
		return err
	}
	src.Location = loc
	// Resolved bytes are literal: mark so re-resolution is a no-op (the
	// lenient branch keeps the template, so resolved is false and the
	// marker stays unset); see Source.SecretsResolved.
	src.SecretsResolved = resolved
	return nil
}

// maybeExpandCollection returns coll unchanged when --expand is not set
// on cmd. Otherwise it returns a deep clone whose source Locations have
// each been passed through expandSourceLocation (lenient on resolver
// error, strict on parse error and context cancellation; see
// expandLocation). Already-resolved sources are skipped.
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
		if err := expandSourceLocation(ctx, ru, src); err != nil {
			return nil, err
		}
	}
	return clone, nil
}

// maybeExpandSource is the single-source variant of
// maybeExpandCollection. Same semantics: --expand unset returns input
// verbatim; --expand set returns a cloned source with the location
// expanded, lenient on resolver error, propagating parse errors and
// context cancellation. A source already carrying a resolved literal
// location (Source.SecretsResolved) is returned unchanged.
func maybeExpandSource(ctx context.Context, ru *run.Run, cmd *cobra.Command,
	src *source.Source,
) (*source.Source, error) {
	if !cmdFlagIsSetTrue(cmd, flag.Expand) || src == nil || src.SecretsResolved {
		return src, nil
	}

	clone := src.Clone()
	if err := expandSourceLocation(ctx, ru, clone); err != nil {
		return nil, err
	}
	return clone, nil
}

// maybeExpandGroup is the source.Group variant of maybeExpandSource.
// When --expand is set on cmd, it returns a clone of g whose sources
// (recursively, including subgroups) have been expanded via
// maybeExpandSource. Mirrors source.RedactGroup, which performs the
// same walk for the redaction step.
func maybeExpandGroup(ctx context.Context, ru *run.Run, cmd *cobra.Command,
	g *source.Group,
) (*source.Group, error) {
	if !cmdFlagIsSetTrue(cmd, flag.Expand) || g == nil {
		return g, nil
	}

	clone := &source.Group{Name: g.Name, Active: g.Active}
	if g.Sources != nil {
		clone.Sources = make([]*source.Source, len(g.Sources))
		for i, src := range g.Sources {
			expanded, err := maybeExpandSource(ctx, ru, cmd, src)
			if err != nil {
				return nil, err
			}
			clone.Sources[i] = expanded
		}
	}
	if g.Groups != nil {
		clone.Groups = make([]*source.Group, len(g.Groups))
		for i, sub := range g.Groups {
			expanded, err := maybeExpandGroup(ctx, ru, cmd, sub)
			if err != nil {
				return nil, err
			}
			clone.Groups[i] = expanded
		}
	}
	return clone, nil
}
