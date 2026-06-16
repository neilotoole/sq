package cli

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// countingResolver wraps stubResolver, counting Resolve invocations.
type countingResolver struct {
	stub  *stubResolver
	count atomic.Int64
}

func (c *countingResolver) Resolve(ctx context.Context, path string) (string, error) {
	c.count.Add(1)
	return c.stub.Resolve(ctx, path)
}

// recordingSourceWriter is a fake output.SourceWriter that records the
// values it receives.
type recordingSourceWriter struct {
	gotColl  *source.Collection
	gotSrcs  []*source.Source
	gotGroup *source.Group
}

var _ output.SourceWriter = (*recordingSourceWriter)(nil)

func (w *recordingSourceWriter) Collection(coll *source.Collection) error {
	w.gotColl = coll
	return nil
}

func (w *recordingSourceWriter) Source(_ *source.Collection, src *source.Source) error {
	w.gotSrcs = append(w.gotSrcs, src)
	return nil
}

func (w *recordingSourceWriter) Added(_ *source.Collection, src *source.Source) error {
	w.gotSrcs = append(w.gotSrcs, src)
	return nil
}

func (w *recordingSourceWriter) Removed(srcs ...*source.Source) error {
	w.gotSrcs = append(w.gotSrcs, srcs...)
	return nil
}

func (w *recordingSourceWriter) Moved(_ *source.Collection, old, nu *source.Source) error {
	w.gotSrcs = append(w.gotSrcs, old, nu)
	return nil
}

func (w *recordingSourceWriter) Group(group *source.Group) error {
	w.gotGroup = group
	return nil
}

func (w *recordingSourceWriter) SetActiveGroup(group *source.Group) error {
	w.gotGroup = group
	return nil
}

func (w *recordingSourceWriter) Groups(tree *source.Group) error {
	w.gotGroup = tree
	return nil
}

// recordingPingWriter is a fake output.PingWriter that records the
// sources it receives.
type recordingPingWriter struct {
	gotOpen   []*source.Source
	gotResult []*source.Source
}

var _ output.PingWriter = (*recordingPingWriter)(nil)

func (w *recordingPingWriter) Open(srcs []*source.Source) error {
	w.gotOpen = srcs
	return nil
}

func (w *recordingPingWriter) Result(src *source.Source, _ time.Duration, _ error) error {
	w.gotResult = append(w.gotResult, src)
	return nil
}

func (w *recordingPingWriter) Close() error { return nil }

// recordingMetadataWriter is a fake output.MetadataWriter that records
// the source metadata it receives.
type recordingMetadataWriter struct {
	gotSrcMeta *metadata.Source
}

var _ output.MetadataWriter = (*recordingMetadataWriter)(nil)

func (w *recordingMetadataWriter) TableMetadata(*metadata.Table) error { return nil }

func (w *recordingMetadataWriter) SourceMetadata(srcMeta *metadata.Source, _ bool) error {
	w.gotSrcMeta = srcMeta
	return nil
}

func (w *recordingMetadataWriter) DBProperties(map[string]any) error      { return nil }
func (w *recordingMetadataWriter) DriverMetadata([]driver.Metadata) error { return nil }
func (w *recordingMetadataWriter) Catalogs(string, []string) error        { return nil }
func (w *recordingMetadataWriter) Schemata(string, []*metadata.Schema) error {
	return nil
}

// newExpanderCmd returns a cmd suitable for the expand decorators: the
// --expand flag is registered (and set per the set arg), and the cmd's
// context carries a run.Run whose SecretRegistry "keyring" scheme is
// backed by values.
func newExpanderCmd(t *testing.T, set bool, values map[string]string) *cobra.Command {
	t.Helper()
	cmd := newCmdWithExpand(t, set)
	ru := newTestRun(t, values)
	cmd.SetContext(run.NewContext(context.Background(), ru))
	return cmd
}

func TestExpandSourceWriter_Source_FlagSet_Expands(t *testing.T) {
	cmd := newExpanderCmd(t, true, map[string]string{
		"abc": "postgres://alice:hunter2@db/sakila",
	})

	rec := &recordingSourceWriter{}
	ew := &expandSourceWriter{w: rec, expander: expander{cmd: cmd}}

	src := &source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}
	require.NoError(t, ew.Source(nil, src))
	require.Len(t, rec.gotSrcs, 1)
	require.Equal(t, "postgres://alice:hunter2@db/sakila", rec.gotSrcs[0].Location)
	require.Equal(t, "${keyring:abc}", src.Location, "input must not be mutated")
}

func TestExpandSourceWriter_Source_FlagUnset_PassThrough(t *testing.T) {
	cmd := newExpanderCmd(t, false, nil)

	rec := &recordingSourceWriter{}
	ew := &expandSourceWriter{w: rec, expander: expander{cmd: cmd}}

	src := &source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}
	require.NoError(t, ew.Source(nil, src))
	require.Len(t, rec.gotSrcs, 1)
	require.Same(t, src, rec.gotSrcs[0],
		"flag unset must pass input through verbatim")
}

func TestExpandSourceWriter_ParseErrorPropagates(t *testing.T) {
	cmd := newExpanderCmd(t, true, nil)

	rec := &recordingSourceWriter{}
	ew := &expandSourceWriter{w: rec, expander: expander{cmd: cmd}}

	src := &source.Source{
		Handle:   "@bad",
		Type:     drivertype.Pg,
		Location: "${malformed", // Unclosed brace: parse error.
	}
	err := ew.Source(nil, src)
	require.Error(t, err, "malformed placeholder must surface as an error")
	require.Contains(t, err.Error(), "@bad")
	require.Empty(t, rec.gotSrcs, "delegate must not be invoked on error")
}

func TestExpandSourceWriter_Collection_Expands(t *testing.T) {
	cmd := newExpanderCmd(t, true, map[string]string{
		"abc": "postgres://alice:hunter2@db/sakila",
	})

	rec := &recordingSourceWriter{}
	ew := &expandSourceWriter{w: rec, expander: expander{cmd: cmd}}

	coll := &source.Collection{}
	require.NoError(t, coll.Add(&source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}))

	require.NoError(t, ew.Collection(coll))
	require.NotNil(t, rec.gotColl)
	require.NotSame(t, coll, rec.gotColl, "must clone when --expand is set")
	require.Equal(t, "postgres://alice:hunter2@db/sakila",
		rec.gotColl.Sources()[0].Location)
	require.Equal(t, "${keyring:abc}", coll.Sources()[0].Location,
		"input must not be mutated")
}

func TestExpandSourceWriter_Group_ExpandsNestedSources(t *testing.T) {
	cmd := newExpanderCmd(t, true, map[string]string{
		"abc": "postgres://alice:hunter2@db/sakila",
		"def": "mysql://bob:opensesame@db/sakila",
	})

	rec := &recordingSourceWriter{}
	ew := &expandSourceWriter{w: rec, expander: expander{cmd: cmd}}

	group := &source.Group{
		Name: "/",
		Sources: []*source.Source{
			{Handle: "@a", Type: drivertype.Pg, Location: "${keyring:abc}"},
		},
		Groups: []*source.Group{
			{
				Name: "prod",
				Sources: []*source.Source{
					{Handle: "@prod/b", Type: drivertype.MySQL, Location: "${keyring:def}"},
				},
			},
		},
	}

	require.NoError(t, ew.Group(group))
	require.NotNil(t, rec.gotGroup)
	require.NotSame(t, group, rec.gotGroup)
	require.Equal(t, "postgres://alice:hunter2@db/sakila",
		rec.gotGroup.Sources[0].Location)
	require.Equal(t, "mysql://bob:opensesame@db/sakila",
		rec.gotGroup.Groups[0].Sources[0].Location,
		"subgroup sources must expand too")
	require.Equal(t, "${keyring:abc}", group.Sources[0].Location,
		"input must not be mutated")
	require.Equal(t, "${keyring:def}", group.Groups[0].Sources[0].Location,
		"input subgroup must not be mutated")
}

// TestExpandPingWriter_OpenCachesForResult verifies that the ping
// decorator resolves each source exactly once: Open expands and caches,
// and Result reuses the cached expansion instead of repeating
// resolver I/O.
func TestExpandPingWriter_OpenCachesForResult(t *testing.T) {
	counting := &countingResolver{stub: &stubResolver{values: map[string]string{
		"abc": "postgres://alice:hunter2@db/sakila",
	}}}
	reg := secret.NewRegistry()
	reg.Register("keyring", counting)
	ru := &run.Run{SecretRegistry: reg}
	cmd := newCmdWithExpand(t, true)
	cmd.SetContext(run.NewContext(context.Background(), ru))

	rec := &recordingPingWriter{}
	ew := &expandPingWriter{w: rec, expander: expander{cmd: cmd}}

	src := &source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}

	require.NoError(t, ew.Open([]*source.Source{src}))
	require.Len(t, rec.gotOpen, 1)
	require.Equal(t, "postgres://alice:hunter2@db/sakila", rec.gotOpen[0].Location)

	require.NoError(t, ew.Result(src, time.Millisecond, nil))
	require.Len(t, rec.gotResult, 1)
	require.Equal(t, "postgres://alice:hunter2@db/sakila", rec.gotResult[0].Location)

	require.Equal(t, int64(1), counting.count.Load(),
		"resolver must be invoked exactly once per source (Open caches for Result)")
}

func TestExpandMetadataWriter_SourceMetadata_Expands(t *testing.T) {
	cmd := newExpanderCmd(t, true, map[string]string{
		"abc": "postgres://alice:hunter2@db/sakila",
	})

	rec := &recordingMetadataWriter{}
	ew := &expandMetadataWriter{w: rec, expander: expander{cmd: cmd}}

	srcMeta := &metadata.Source{
		Handle:   "@a",
		Location: "${keyring:abc}",
	}
	require.NoError(t, ew.SourceMetadata(srcMeta, true))
	require.NotNil(t, rec.gotSrcMeta)
	require.Equal(t, "postgres://alice:hunter2@db/sakila", rec.gotSrcMeta.Location)
	require.Equal(t, "${keyring:abc}", srcMeta.Location,
		"input must not be mutated")
}

// TestExpandMetadataWriter_SecretsResolved_Skipped verifies the metadata
// decorator skips expansion for an already-resolved location, matching
// the source/group/collection paths, so a literal '$$' is not
// double-unescaped.
func TestExpandMetadataWriter_SecretsResolved_Skipped(t *testing.T) {
	cmd := newExpanderCmd(t, true, nil)

	rec := &recordingMetadataWriter{}
	ew := &expandMetadataWriter{w: rec, expander: expander{cmd: cmd}}

	srcMeta := &metadata.Source{
		Handle:          "@a",
		Location:        "postgres://b:pa$$wd@h/db",
		SecretsResolved: true,
	}
	require.NoError(t, ew.SourceMetadata(srcMeta, true))
	require.NotNil(t, rec.gotSrcMeta)
	require.Equal(t, "postgres://b:pa$$wd@h/db", rec.gotSrcMeta.Location,
		"already-resolved location must not be re-unescaped")
}

// TestExpander_NoRunOnContext_NoPanic verifies that an expand decorator
// over a command whose context carries no run does not panic when
// --expand is set: active() returns false and the source passes through
// unchanged. Guards the runCtx -> run.FromContextOrNil contract.
func TestExpander_NoRunOnContext_NoPanic(t *testing.T) {
	cmd := newCmdWithExpand(t, true)
	cmd.SetContext(context.Background()) // no run installed

	// Assert the flag is set, so active()==false below is necessarily due
	// to the missing run (not an unset flag), making this a real test of
	// the FromContextOrNil no-panic path.
	require.True(t, cmdFlagIsSetTrue(cmd, flag.Expand))

	e := expander{cmd: cmd}
	require.False(t, e.active(), "active must be false when no run is on the context")

	src := &source.Source{Handle: "@a", Location: "${keyring:abc}"}
	got, err := e.src(src)
	require.NoError(t, err)
	require.Same(t, src, got, "no run -> pass through unchanged, no panic")
}
