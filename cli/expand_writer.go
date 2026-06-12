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
// The decorators hold the cobra command and derive the context, run
// (and thus the secret registry), and flag state at write time, so
// context updates made by the command (e.g. cmd.SetContext) are
// honored.

// expander is the shared core of the expand decorators.
type expander struct {
	cmd *cobra.Command
}

// runCtx returns the command's current context and run. The returned
// run may be nil in degenerate cases (e.g. a test harness without a
// run on the context); callers must treat nil as "don't expand".
func (e expander) runCtx() (context.Context, *run.Run) {
	ctx := e.cmd.Context()
	ru := run.FromContext(ctx)
	if ru == nil || ru.SecretRegistry == nil {
		return ctx, nil
	}
	return ctx, ru
}

// active reports whether expansion should be attempted at all.
func (e expander) active() bool {
	if !cmdFlagIsSetTrue(e.cmd, flag.Expand) {
		return false
	}
	_, ru := e.runCtx()
	return ru != nil
}

// src returns the expanded clone of s per maybeExpandSource.
func (e expander) src(s *source.Source) (*source.Source, error) {
	if !e.active() {
		return s, nil
	}
	ctx, ru := e.runCtx()
	return maybeExpandSource(ctx, ru, e.cmd, s)
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
	if ew.active() {
		expanded := make([]*source.Source, len(srcs))
		for i, src := range srcs {
			exp, err := ew.src(src)
			if err != nil {
				return err
			}
			expanded[i] = exp
		}
		srcs = expanded
	}
	return ew.w.Removed(srcs...)
}

// Moved implements output.SourceWriter.
func (ew *expandSourceWriter) Moved(coll *source.Collection, old, nu *source.Source) error {
	old, err := ew.src(old)
	if err != nil {
		return err
	}
	if nu, err = ew.src(nu); err != nil {
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
	if ew.active() {
		ew.cache = make(map[*source.Source]*source.Source, len(srcs))
		expanded := make([]*source.Source, len(srcs))
		for i, src := range srcs {
			exp, err := ew.src(src)
			if err != nil {
				return err
			}
			expanded[i] = exp
			ew.cache[src] = exp
		}
		srcs = expanded
	}
	return ew.w.Open(srcs)
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
	return ew.w.Close()
}

// expandMetadataWriter decorates an output.MetadataWriter, expanding
// the location in SourceMetadata per the --expand flag before
// delegating. The other MetadataWriter methods carry no source
// location and delegate unchanged.
//
// Note that callers are expected to populate srcMeta.Location with the
// stored (template) location: drivers populate it from the grip's
// resolved source, which execInspect resets to the stored template
// before writing.
type expandMetadataWriter struct {
	w output.MetadataWriter
	expander
}

var _ output.MetadataWriter = (*expandMetadataWriter)(nil)

// SourceMetadata implements output.MetadataWriter.
func (ew *expandMetadataWriter) SourceMetadata(srcMeta *metadata.Source, showSchema bool) error {
	if srcMeta != nil && ew.active() {
		ctx, ru := ew.runCtx()
		loc, _, err := expandLocation(ctx, ru, srcMeta.Handle, srcMeta.Location)
		if err != nil {
			return err
		}
		clone := *srcMeta
		clone.Location = loc
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
