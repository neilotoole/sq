package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// This file contains decorators that apply --expand location expansion
// in the writer layer, mirroring how redaction is enforced once via
// Printing.Redact rather than per command. newWriters wraps the writer
// interfaces that print source locations (SourceWriter, PingWriter,
// MetadataWriter) so that ANY command printing a location honors
// --expand, including future commands that don't know the flag exists.
//
// Why decorators in cli, not an Expand field on output.Printing
// consumed by each writer impl? Unlike redaction, expansion performs
// fallible I/O (keyring, 1Password CLI, file, env resolvers). Pushing
// that into every format writer (tablew, jsonw, yamlw, ...) would
// duplicate the I/O and error plumbing across implementations; a
// single decorator runs the expansion exactly once per write and
// propagates failures through the writer methods' existing error
// returns, so there's no silent partial expansion beyond the
// deliberate lenient-resolver fallback (see expandLocation).
//
// Why not expand eagerly in preRun? Expanding ru.Config.Collection in
// place would risk persisting resolved secrets when a command saves
// config (add, mv, src), and would miss sources created after preRun
// (e.g. stdin sources). Lazily expanding cloned values at write time
// has neither problem.
//
// The decorators hold the cobra command and the run (injected at
// construction by newWriters, which carries the secret registry). The
// context and --expand flag state are read from the command fresh at
// write time, so context updates made by the command (e.g. cmd.SetContext
// timeouts) are honored when the resolver runs.

// expander is the shared core of the expand decorators. The run is
// injected at construction (newWriters), where it's always in hand; the
// context is read fresh from cmd at write time so cmd.SetContext updates
// (e.g. timeouts) are honored when the resolver runs.
type expander struct {
	cmd *cobra.Command
	ru  *run.Run
}

// runCtx returns the command's current context (read fresh so
// cmd.SetContext updates such as timeouts are honored when the resolver
// runs) and the injected run.
func (e expander) runCtx() (context.Context, *run.Run) {
	return e.cmd.Context(), e.ru
}

// active reports whether expansion should be attempted (the --expand flag
// is set). The run and its secret registry are injected at construction
// and always present by the time a decorator runs, so there's nothing
// else to gate on.
func (e expander) active() bool {
	return cmdFlagIsSetTrue(e.cmd, flag.Expand)
}

// src returns the expanded clone of s per maybeExpandSource.
func (e expander) src(s *source.Source) (*source.Source, error) {
	if !e.active() {
		return s, nil
	}
	ctx, ru := e.runCtx()
	return maybeExpandSource(ctx, ru, e.cmd, s)
}

// srcs returns expanded clones of each element of in per src. When
// expansion is inactive, in is returned unchanged.
func (e expander) srcs(in []*source.Source) ([]*source.Source, error) {
	if !e.active() {
		return in, nil
	}
	out := make([]*source.Source, len(in))
	for i, s := range in {
		exp, err := e.src(s)
		if err != nil {
			return nil, err
		}
		out[i] = exp
	}
	return out, nil
}

// expandSourceWriter decorates an output.SourceWriter, expanding
// source locations per the --expand flag before delegating.
type expandSourceWriter struct {
	w output.SourceWriter
	expander
}

var _ output.SourceWriter = (*expandSourceWriter)(nil)

// Collection implements output.SourceWriter.
func (ew *expandSourceWriter) Collection(coll *source.Collection) error {
	if ew.active() {
		ctx, ru := ew.runCtx()
		var err error
		if coll, err = maybeExpandCollection(ctx, ru, ew.cmd, coll); err != nil {
			return err
		}
	}
	return ew.w.Collection(coll)
}

// Source implements output.SourceWriter.
func (ew *expandSourceWriter) Source(coll *source.Collection, src *source.Source) error {
	src, err := ew.src(src)
	if err != nil {
		return err
	}
	return ew.w.Source(coll, src)
}

// Added implements output.SourceWriter.
func (ew *expandSourceWriter) Added(coll *source.Collection, src *source.Source) error {
	src, err := ew.src(src)
	if err != nil {
		return err
	}
	return ew.w.Added(coll, src)
}

// Removed implements output.SourceWriter.
func (ew *expandSourceWriter) Removed(srcs ...*source.Source) error {
	srcs, err := ew.srcs(srcs)
	if err != nil {
		return err
	}
	return ew.w.Removed(srcs...)
}

// Moved implements output.SourceWriter. Only nu is expanded: every
// underlying Moved impl displays the destination source and discards
// old, so expanding old would be wasted resolver I/O on a value no
// writer prints.
func (ew *expandSourceWriter) Moved(coll *source.Collection, old, nu *source.Source) error {
	nu, err := ew.src(nu)
	if err != nil {
		return err
	}
	return ew.w.Moved(coll, old, nu)
}

// Group implements output.SourceWriter.
func (ew *expandSourceWriter) Group(group *source.Group) error {
	group, err := ew.group(group)
	if err != nil {
		return err
	}
	return ew.w.Group(group)
}

// SetActiveGroup implements output.SourceWriter.
func (ew *expandSourceWriter) SetActiveGroup(group *source.Group) error {
	group, err := ew.group(group)
	if err != nil {
		return err
	}
	return ew.w.SetActiveGroup(group)
}

// Groups implements output.SourceWriter.
func (ew *expandSourceWriter) Groups(tree *source.Group) error {
	tree, err := ew.group(tree)
	if err != nil {
		return err
	}
	return ew.w.Groups(tree)
}

func (ew *expandSourceWriter) group(g *source.Group) (*source.Group, error) {
	if !ew.active() {
		return g, nil
	}
	ctx, ru := ew.runCtx()
	return maybeExpandGroup(ctx, ru, ew.cmd, g)
}

// expandPingWriter decorates an output.PingWriter, expanding source
// locations per the --expand flag before delegating. Open expands each
// source once and caches the result (keyed by the input pointer), so
// the subsequent per-source Result calls don't repeat resolver I/O.
// PingWriter is invoked sequentially (Open, then Results from a single
// loop; see pingSources), so the cache needs no locking.
type expandPingWriter struct {
	w     output.PingWriter
	cache map[*source.Source]*source.Source
	expander
}

var _ output.PingWriter = (*expandPingWriter)(nil)

// Open implements output.PingWriter.
func (ew *expandPingWriter) Open(srcs []*source.Source) error {
	expanded, err := ew.srcs(srcs)
	if err != nil {
		return err
	}
	// Reset before (maybe) repopulating, so an inactive Open can't leave
	// a stale cache behind.
	ew.cache = nil
	if ew.active() {
		// Cache input->expanded so the per-source Result calls don't
		// repeat resolver I/O (see expandPingWriter doc).
		ew.cache = make(map[*source.Source]*source.Source, len(srcs))
		for i, src := range srcs {
			ew.cache[src] = expanded[i]
		}
	}
	return ew.w.Open(expanded)
}

// Result implements output.PingWriter.
func (ew *expandPingWriter) Result(src *source.Source, d time.Duration, err error) error {
	if exp, ok := ew.cache[src]; ok {
		src = exp
	} else {
		// Defensive: a src that didn't pass through Open.
		var expandErr error
		if src, expandErr = ew.src(src); expandErr != nil {
			return expandErr
		}
	}
	return ew.w.Result(src, d, err)
}

// Close implements output.PingWriter.
func (ew *expandPingWriter) Close() error {
	// Drop the cache once the run is done.
	ew.cache = nil
	return ew.w.Close()
}

// expandMetadataWriter decorates an output.MetadataWriter, expanding
// the location in SourceMetadata per the --expand flag before
// delegating. The other MetadataWriter methods carry no source
// location and delegate unchanged.
//
// Like the source/group expand paths, an already-resolved location is
// skipped (re-resolving a literal would unescape '$$' a second time and
// corrupt it; see maybeExpandSource). metadata.Source.SecretsResolved
// carries that bit: drivers populate srcMeta.Location from the grip's
// resolved source but leave SecretsResolved false; execInspect resets
// the location to the stored template and sets SecretsResolved from the
// source (cmd_inspect.go). Any future SourceMetadata caller that passes
// an already-resolved location must set srcMeta.SecretsResolved so this
// decorator skips it.
//
// Note: because expansion happens at write time, a malformed ${...}
// placeholder under --expand surfaces here (after the source has been
// opened and metadata read) rather than fail-fast before the open. That
// is the accepted cost of lazy, writer-layer expansion; the parse error
// still surfaces.
type expandMetadataWriter struct {
	w output.MetadataWriter
	expander
}

var _ output.MetadataWriter = (*expandMetadataWriter)(nil)

// SourceMetadata implements output.MetadataWriter.
func (ew *expandMetadataWriter) SourceMetadata(srcMeta *metadata.Source, showSchema bool) error {
	if srcMeta != nil && !srcMeta.SecretsResolved && ew.active() {
		ctx, ru := ew.runCtx()
		loc, resolved, err := expandLocation(ctx, ru, srcMeta.Handle, srcMeta.Location)
		if err != nil {
			return err
		}
		clone := *srcMeta
		clone.Location = loc
		// Keep the clone self-consistent: a resolved literal location gets
		// the resolved marker (the lenient branch keeps the template and
		// leaves resolved false; see expandLocation).
		clone.SecretsResolved = resolved
		srcMeta = &clone
	}
	return ew.w.SourceMetadata(srcMeta, showSchema)
}

// TableMetadata implements output.MetadataWriter.
func (ew *expandMetadataWriter) TableMetadata(tblMeta *metadata.Table) error {
	return ew.w.TableMetadata(tblMeta)
}

// DBProperties implements output.MetadataWriter.
func (ew *expandMetadataWriter) DBProperties(props map[string]any) error {
	return ew.w.DBProperties(props)
}

// DriverMetadata implements output.MetadataWriter.
func (ew *expandMetadataWriter) DriverMetadata(drvrs []driver.Metadata) error {
	return ew.w.DriverMetadata(drvrs)
}

// Catalogs implements output.MetadataWriter.
func (ew *expandMetadataWriter) Catalogs(currentCatalog string, catalogs []string) error {
	return ew.w.Catalogs(currentCatalog, catalogs)
}

// Schemata implements output.MetadataWriter.
func (ew *expandMetadataWriter) Schemata(currentSchema string, schemas []*metadata.Schema) error {
	return ew.w.Schemata(currentSchema, schemas)
}
