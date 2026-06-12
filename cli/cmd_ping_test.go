package cli_test

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/core/secret"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
	"github.com/neilotoole/sq/testh/sakila"
)

func TestCmdPing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	err := testrun.New(ctx, t, nil).Exec("ping")
	require.Error(t, err, "no active data source")

	err = testrun.New(ctx, t, nil).Exec("ping", "invalid_handle")
	require.Error(t, err)

	err = testrun.New(ctx, t, nil).Exec("ping", "@not_a_handle")
	require.Error(t, err)

	var tr *testrun.TestRun

	th := testh.New(t)
	src1, src2 := th.Source(sakila.CSVActor), th.Source(sakila.CSVActorNoHeader)

	tr = testrun.New(ctx, t, nil).Add(*src1)
	err = tr.Exec("ping", "--csv", src1.Handle)
	require.NoError(t, err)
	checkPingOutputCSV(t, tr, *src1)

	tr = testrun.New(ctx, t, nil).Add(*src2)
	err = tr.Exec("ping", "--csv", src2.Handle)
	require.NoError(t, err)
	checkPingOutputCSV(t, tr, *src2)

	tr = testrun.New(ctx, t, nil).Add(*src1, *src2)
	err = tr.Exec("ping", "--csv", src1.Handle, src2.Handle)
	require.NoError(t, err)
	checkPingOutputCSV(t, tr, *src1, *src2)
}

// checkPintOutputCSV reads CSV records from h.out, and verifies
// that there's an appropriate record for each of srcs.
func checkPingOutputCSV(t *testing.T, h *testrun.TestRun, srcs ...source.Source) {
	t.Helper()
	recs, err := csv.NewReader(h.Out).ReadAll()
	require.NoError(t, err)
	require.Equal(t, len(srcs), len(recs))

	if len(srcs) > 0 {
		require.Equal(t, 3, len(recs[0]), "each ping record should have 3 fields, but got %d fields", len(recs[0]))
	}

	handles := make(map[string]bool)
	for _, src := range srcs {
		handles[src.Handle] = true
	}

	for i := range recs {
		recHandle := recs[i][0]
		require.True(t, handles[recHandle], "should have handle %s in map", recHandle)

		_, err = time.ParseDuration(recs[i][1])
		require.NoError(t, err, "should be a valid duration value")

		require.Equal(t, "pong", recs[i][2], "error field should be empty")
	}
}

// TestCmdPing_KeyringPlaceholder_NoLeakInJSON verifies that sq ping --json
// never emits a plaintext password, regardless of whether the source
// Location holds a keyring placeholder or an already-resolved DSN.
//
// When redaction is on (the default), a placeholder in the password
// position is masked just like an inline password: the output shows
// "xxxxx", not the resolved secret. That is the correct behavior: the
// ping writer applies RedactedLocation before serializing, so neither
// the resolved plaintext nor the raw placeholder leaks.
func TestCmdPing_KeyringPlaceholder_NoLeakInJSON(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@px_leak/password", "totally-secret-pw"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@px_leak",
		Type:     drivertype.Pg,
		Location: "postgres://alice:${keyring:@px_leak/password}@127.0.0.1:1/sakila",
	}))

	// Ping will fail at the connect step; we don't care about success,
	// we care that the resolved plaintext does NOT appear and that the
	// output is redacted (xxxxx in the password slot).
	_ = tr.Exec("ping", "@px_leak", "--json")
	out := tr.Out.String()
	require.NotContains(t, out, "totally-secret-pw",
		"resolved password must not leak into ping output")
	require.Contains(t, out, "xxxxx",
		"password slot must be redacted in default (non-reveal) mode")
}

// TestCmdPing_Expand_EscapedLocation verifies that `sq ping --expand`
// resolves the location template exactly once. The driver must connect
// using the original stored source (resolved inside pingSource);
// feeding the already-expanded source back through
// ResolveSourceSecrets would unescape '$$' a second time, corrupting
// literal locations the v0.54.0 upgrade escaped.
func TestCmdPing_Expand_EscapedLocation(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "data$$file.csv")
	require.NoError(t, os.WriteFile(fpath, []byte("a,b\n1,2\n"), 0o600))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle: "@csv_dollar",
		Type:   drivertype.CSV,
		// Stored template form: '$' escaped as '$$', as the v0.54.0
		// config upgrade writes it.
		Location: secret.Escape(fpath),
	}))

	require.NoError(t, tr.Exec("ping", "--expand", "@csv_dollar"),
		"ping --expand must resolve the location exactly once (double-unescape breaks the path)")

	// Sanity: plain ping (no --expand) also works.
	tr = testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@csv_dollar",
		Type:     drivertype.CSV,
		Location: secret.Escape(fpath),
	}))
	require.NoError(t, tr.Exec("ping", "@csv_dollar"))
}

// TestCmdPing_KeyringMissing_DoesNotPanic guards against the nil-panic that
// happened when ResolveSourceSecrets returned (nil, err) and the channel
// carried a nil src to the output writer.
func TestCmdPing_KeyringMissing_DoesNotPanic(t *testing.T) {
	gokeyring.MockInit()
	// Deliberately do NOT set the keyring entry.

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@px_missing",
		Type:     drivertype.Pg,
		Location: "postgres://alice:${keyring:@px_missing/password}@127.0.0.1:1/sakila",
	}))

	// Must not panic; should return an error.
	err := tr.Exec("ping", "@px_missing", "--json")
	require.Error(t, err)
}
