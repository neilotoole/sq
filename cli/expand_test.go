package cli

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// stubResolver returns canned values keyed by path. Missing path
// yields secret.ErrNotFound. If ctx is canceled, Resolve returns
// ctx.Err() so the cancellation-propagation test can exercise that
// path without involving a real resolver.
type stubResolver struct {
	values map[string]string
}

func (s *stubResolver) Resolve(ctx context.Context, path string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	v, ok := s.values[path]
	if !ok {
		return "", secret.ErrNotFound
	}
	return v, nil
}

// newTestRun returns a *run.Run with a SecretRegistry whose "keyring"
// scheme is backed by stubResolver{values}. Other schemes are
// unregistered (any reference to them returns ErrUnknownScheme).
func newTestRun(t *testing.T, values map[string]string) *run.Run {
	t.Helper()
	reg := secret.NewRegistry()
	reg.Register("keyring", &stubResolver{values: values})
	return &run.Run{SecretRegistry: reg}
}

// newCmdWithExpand returns a *cobra.Command with --expand registered
// on its local flag set. When set is true, the flag is marked Changed
// to true via Flags().Set so cmdFlagIsSetTrue returns true. (See
// cli/flags.go:47: cmdFlagIsSetTrue requires Changed AND value=true.)
func newCmdWithExpand(t *testing.T, set bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().Bool(flag.Expand, false, flag.ExpandUsage)
	if set {
		require.NoError(t, cmd.Flags().Set(flag.Expand, "true"))
	}
	return cmd
}

func TestMaybeExpandCollection_FlagUnset_ReturnsInputUnchanged(t *testing.T) {
	ru := newTestRun(t, nil)
	cmd := newCmdWithExpand(t, false)

	coll := &source.Collection{}
	require.NoError(t, coll.Add(&source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}))

	got, err := maybeExpandCollection(context.Background(), ru, cmd, coll)
	require.NoError(t, err)
	require.Same(t, coll, got, "must return input verbatim when --expand is not set")
}

func TestMaybeExpandCollection_FlagSet_ExpandsKeyring(t *testing.T) {
	ru := newTestRun(t, map[string]string{
		"abc": "postgres://alice:hunter2@db/sakila",
	})
	cmd := newCmdWithExpand(t, true)

	coll := &source.Collection{}
	require.NoError(t, coll.Add(&source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}))

	got, err := maybeExpandCollection(context.Background(), ru, cmd, coll)
	require.NoError(t, err)
	require.NotSame(t, coll, got, "must clone when --expand is set")
	require.Equal(t, "postgres://alice:hunter2@db/sakila", got.Sources()[0].Location)
	require.Equal(t, "${keyring:abc}", coll.Sources()[0].Location,
		"input must not be mutated")
}

func TestMaybeExpandCollection_FlagSet_LenientOnResolverError(t *testing.T) {
	ru := newTestRun(t, map[string]string{
		"def": "postgres://b:pw@h/db",
		// "abc" intentionally not set.
	})
	cmd := newCmdWithExpand(t, true)

	coll := &source.Collection{}
	require.NoError(t, coll.Add(&source.Source{
		Handle:   "@missing",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}))
	require.NoError(t, coll.Add(&source.Source{
		Handle:   "@ok",
		Type:     drivertype.Pg,
		Location: "${keyring:def}",
	}))

	got, err := maybeExpandCollection(context.Background(), ru, cmd, coll)
	require.NoError(t, err, "per-source resolver error must be swallowed")

	srcs := got.Sources()
	require.Equal(t, "${keyring:abc}", srcs[0].Location,
		"unresolvable placeholder must be left verbatim")
	require.Equal(t, "postgres://b:pw@h/db", srcs[1].Location,
		"resolvable placeholder must still expand")
}

func TestMaybeExpandCollection_ContextCanceled_Propagates(t *testing.T) {
	ru := newTestRun(t, map[string]string{
		"abc": "postgres://x:y@h/db",
	})
	cmd := newCmdWithExpand(t, true)

	coll := &source.Collection{}
	require.NoError(t, coll.Add(&source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := maybeExpandCollection(ctx, ru, cmd, coll)
	require.True(t, errors.Is(err, context.Canceled),
		"context.Canceled must propagate, not be swallowed as a per-source error")
}

func TestMaybeExpandSource_FlagUnset_ReturnsInputUnchanged(t *testing.T) {
	ru := newTestRun(t, nil)
	cmd := newCmdWithExpand(t, false)

	src := &source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}

	got, err := maybeExpandSource(context.Background(), ru, cmd, src)
	require.NoError(t, err)
	require.Same(t, src, got)
}

func TestMaybeExpandSource_FlagSet_Expands(t *testing.T) {
	ru := newTestRun(t, map[string]string{
		"abc": "postgres://alice:hunter2@db/sakila",
	})
	cmd := newCmdWithExpand(t, true)

	src := &source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}

	got, err := maybeExpandSource(context.Background(), ru, cmd, src)
	require.NoError(t, err)
	require.NotSame(t, src, got, "must clone")
	require.Equal(t, "postgres://alice:hunter2@db/sakila", got.Location)
	require.Equal(t, "${keyring:abc}", src.Location, "input must not be mutated")
}

func TestMaybeExpandSource_FlagSet_LenientOnResolverError(t *testing.T) {
	ru := newTestRun(t, nil) // "abc" not set.
	cmd := newCmdWithExpand(t, true)

	src := &source.Source{
		Handle:   "@a",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}

	got, err := maybeExpandSource(context.Background(), ru, cmd, src)
	require.NoError(t, err)
	require.Equal(t, "${keyring:abc}", got.Location,
		"unresolvable placeholder must be left verbatim")
}

func TestMaybeExpandCollection_ParseErrorPropagates(t *testing.T) {
	ru := newTestRun(t, nil)
	cmd := newCmdWithExpand(t, true)

	coll := &source.Collection{}
	require.NoError(t, coll.Add(&source.Source{
		Handle:   "@bad",
		Type:     drivertype.Pg,
		Location: "${malformed", // unclosed brace: parse error.
	}))

	_, err := maybeExpandCollection(context.Background(), ru, cmd, coll)
	require.Error(t, err, "malformed placeholder must surface as an error")
	require.Contains(t, err.Error(), "@bad",
		"error message must include the source handle")
}

func TestMaybeExpandSource_ParseErrorPropagates(t *testing.T) {
	ru := newTestRun(t, nil)
	cmd := newCmdWithExpand(t, true)

	src := &source.Source{
		Handle:   "@bad",
		Type:     drivertype.Pg,
		Location: "${malformed", // unclosed brace: parse error.
	}

	_, err := maybeExpandSource(context.Background(), ru, cmd, src)
	require.Error(t, err, "malformed placeholder must surface as an error")
	require.Contains(t, err.Error(), "@bad",
		"error message must include the source handle")
}

// captureHandler is a minimal slog.Handler that records log entries for
// test assertions. It captures all levels.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	// Simplified: ignore attrs for test purposes.
	return h
}

func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }

func (h *captureHandler) hasMessage(msg string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, r := range h.records {
		if strings.Contains(r.Message, msg) {
			return true
		}
	}
	return false
}

func TestMaybeExpandCollection_DebugLogsSwallowedError(t *testing.T) {
	ru := newTestRun(t, map[string]string{
		// "abc" intentionally not set, so the resolver returns ErrNotFound.
	})
	cmd := newCmdWithExpand(t, true)

	coll := &source.Collection{}
	require.NoError(t, coll.Add(&source.Source{
		Handle:   "@missing",
		Type:     drivertype.Pg,
		Location: "${keyring:abc}",
	}))

	handler := &captureHandler{}
	log := slog.New(handler)
	ctx := lg.NewContext(context.Background(), log)

	_, err := maybeExpandCollection(ctx, ru, cmd, coll)
	require.NoError(t, err, "resolver miss must still be swallowed")
	require.True(t, handler.hasMessage("expand: leaving placeholder verbatim"),
		"swallowed resolver error must emit a debug log entry")
}
