package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/testrun"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/testh"
)

func TestCmdConfigSecretsSet_ExplicitValue(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	err := tr.Exec("config", "secrets", "set", "@sakila/password", "hunter2")
	require.NoError(t, err)

	got, err := gokeyring.Get("sq", "@sakila/password")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestCmdConfigSecretsSet_PromptedFromStdin(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// Pipe a password through stdin; matches the cmd_add test pattern.
	tmp, err := os.CreateTemp(t.TempDir(), "pw")
	require.NoError(t, err)
	_, err = tmp.WriteString("hunter2\n")
	require.NoError(t, err)
	_, err = tmp.Seek(0, 0)
	require.NoError(t, err)
	tr.Run.Stdin = tmp

	err = tr.Exec("config", "secrets", "set", "@sakila/password", "-p")
	require.NoError(t, err)

	got, err := gokeyring.Get("sq", "@sakila/password")
	require.NoError(t, err)
	require.Equal(t, "hunter2", got)
}

func TestCmdConfigSecretsSet_RequiresValueOrFlag(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// No VALUE arg, no -p flag.
	err := tr.Exec("config", "secrets", "set", "@sakila/password")
	require.Error(t, err)
}

func TestCmdConfigSecretsGet_WithoutRevealPrintsMetadataOnly(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@sakila/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "secrets", "get", "@sakila/password"))
	require.NotContains(t, tr.Out.String(), "hunter2")
	require.Contains(t, tr.Out.String(), "@sakila/password")
}

func TestCmdConfigSecretsGet_WithRevealPrintsValue(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@sakila/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "secrets", "get", "@sakila/password", "--reveal"))
	require.Contains(t, tr.Out.String(), "hunter2")
}

func TestCmdConfigSecretsGet_MissingErrors(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	err := tr.Exec("config", "secrets", "get", "@nope/x")
	require.Error(t, err)
}

func TestCmdConfigSecretsRm(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@sakila/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "secrets", "rm", "@sakila/password"))

	_, err := gokeyring.Get("sq", "@sakila/password")
	require.ErrorIs(t, err, gokeyring.ErrNotFound)
}

func TestCmdConfigSecretsRm_MissingIsNotError(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "secrets", "rm", "@nope/x"))
}

func TestCmdConfigSecretsRm_Aliases(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@a/p", "v"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "secrets", "remove", "@a/p"))

	_, err := gokeyring.Get("sq", "@a/p")
	require.ErrorIs(t, err, gokeyring.ErrNotFound)
}

func TestCmdConfigSecretsTest_AllPass(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@sakila_t/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@sakila_t",
		Type:     "postgres",
		Location: "postgres://alice:${keyring:@sakila_t/password}@db/sakila",
	}))
	// Source with no placeholder: must be a PASS (nothing to resolve).
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@plain_t",
		Type:     "postgres",
		Location: "postgres://alice:hunter2@db/sakila",
	}))

	require.NoError(t, tr.Exec("config", "secrets", "test", "--all"))
	out := tr.Out.String()
	require.Contains(t, out, "@sakila_t")
	require.Contains(t, out, "OK")
	require.Contains(t, out, "@plain_t")
}

func TestCmdConfigSecretsTest_FailureReportsAndErrors(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@good_t/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@good_t",
		Type:     "postgres",
		Location: "postgres://alice:${keyring:@good_t/password}@db/sakila",
	}))
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@missing_t",
		Type:     "postgres",
		Location: "postgres://alice:${keyring:@missing_t/password}@db/sakila",
	}))

	err := tr.Exec("config", "secrets", "test", "--all")
	require.Error(t, err)
	out := tr.Out.String()
	require.Contains(t, out, "@good_t")
	require.Contains(t, out, "OK")
	require.Contains(t, out, "@missing_t")
	require.Contains(t, out, "FAIL")
}

func TestCmdConfigSecretsTest_SingleHandle(t *testing.T) {
	gokeyring.MockInit()
	require.NoError(t, gokeyring.Set("sq", "@one_t/password", "hunter2"))

	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@one_t",
		Type:     "postgres",
		Location: "postgres://alice:${keyring:@one_t/password}@db/sakila",
	}))
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@other_t",
		Type:     "postgres",
		Location: "postgres://alice:${keyring:@other_t/password}@db/sakila",
	}))

	// Test only @one_t — @other_t failure should NOT cause exit error.
	require.NoError(t, tr.Exec("config", "secrets", "test", "@one_t"))
	out := tr.Out.String()
	require.Contains(t, out, "@one_t")
	require.NotContains(t, out, "@other_t")
}

func TestCmdConfigSecretsMigrate_PerCase(t *testing.T) {
	tests := []struct {
		name string
		// inLocation is the source's Location before migrate runs.
		inLocation string
		// wantKeyring is the value the keyring entry should hold after
		// a successful migration (i.e. the full DSN verbatim). Empty
		// when the source should be skipped.
		wantKeyring string
		// wantSkipReason is a substring expected on stdout when the
		// source is skipped. Empty when the source should be migrated.
		wantSkipReason string
	}{
		{
			name:        "url with password",
			inLocation:  "postgres://alice:hunter2@db/sakila",
			wantKeyring: "postgres://alice:hunter2@db/sakila",
		},
		{
			name:           "url without password",
			inLocation:     "postgres://alice@db/sakila",
			wantSkipReason: "no password",
		},
		{
			name:           "non-url",
			inLocation:     "/data/file.xlsx",
			wantSkipReason: "not a URL",
		},
		{
			name: "malformed placeholder is surfaced, not silently migrated",
			// Unclosed ${ — ExtractRefs returns an error. Migrate must
			// NOT stamp the malformed Location into the keyring.
			inLocation:     "postgres://alice:${env:UNCLOSED@db/sakila",
			wantSkipReason: "malformed placeholder",
		},
		{
			name:           "already templated",
			inLocation:     "postgres://alice:${keyring:@h/password}@db/sakila",
			wantSkipReason: "already",
		},
		{
			name:        "url-encoded password preserved verbatim",
			inLocation:  "postgres://alice:p%40ss%3Aword@db/sakila",
			wantKeyring: "postgres://alice:p%40ss%3Aword@db/sakila",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gokeyring.MockInit()
			th := testh.New(t)
			tr := testrun.New(th.Context, t, nil)
			require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
				Handle:   "@h",
				Type:     "postgres",
				Location: tc.inLocation,
			}))

			require.NoError(t, tr.Exec("config", "secrets", "migrate", "--all", "--yes"))

			src, err := tr.Run.Config.Collection.Get("@h")
			require.NoError(t, err)

			if tc.wantSkipReason != "" {
				// Skipped: Location unchanged from input; no keyring entry written.
				require.Equal(t, tc.inLocation, src.Location)
				require.Contains(t, tr.Out.String(), tc.wantSkipReason)
				return
			}

			// Success: Location is a bare ${keyring:<crockford-id>}; keyring at that
			// id holds the entire input DSN verbatim (no URL-decoding, no surgery).
			id := extractKeyringID(t, src.Location)
			got, err := gokeyring.Get("sq", id)
			require.NoError(t, err)
			require.Equal(t, tc.wantKeyring, got)
		})
	}
}

func TestCmdConfigSecretsMigrate_DryRun(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Run.Config.Collection.Add(&source.Source{
		Handle:   "@h_dr",
		Type:     "postgres",
		Location: "postgres://alice:hunter2@db/sakila",
	}))

	require.NoError(t, tr.Exec("config", "secrets", "migrate", "--all", "--dry-run"))

	// Source unchanged.
	src, _ := tr.Run.Config.Collection.Get("@h_dr")
	require.Equal(t, "postgres://alice:hunter2@db/sakila", src.Location)
	// Dry-run mints no IDs and writes nothing to the keyring; the planned
	// output uses the literal "<new-id>" stand-in.
	require.Contains(t, tr.Out.String(), "@h_dr")
	require.Contains(t, tr.Out.String(), "${keyring:<new-id>}")
}

func TestCmdConfigSecretsLs(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)

	// Seed sources with various placeholder shapes.
	tr.Add(
		source.Source{
			Handle:   "@sakila_ls",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:@sakila_ls/password}@db/sakila",
		},
		source.Source{
			Handle:   "@prod_pg_ls",
			Type:     drivertype.Pg,
			Location: "${keyring:@prod_pg_ls/dsn}",
		},
		// Plain inline source — should NOT appear in ls output.
		source.Source{
			Handle:   "@plain_ls",
			Type:     drivertype.Pg,
			Location: "postgres://alice:hunter2@db/sakila",
		},
	)

	require.NoError(t, tr.Exec("config", "secrets", "ls"))
	out := tr.Out.String()
	require.Contains(t, out, "keyring:@sakila_ls/password")
	require.Contains(t, out, "keyring:@prod_pg_ls/dsn")
	require.NotContains(t, out, "@plain_ls")
}

func TestCmdConfigSecretsLs_MultipleSchemes(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Add(
		source.Source{
			Handle:   "@multi_ls",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${env:DB_PW}@${file:/etc/sq/host}/sakila",
		},
		source.Source{
			Handle:   "@multi_ls2",
			Type:     drivertype.Pg,
			Location: "postgres://alice:${keyring:@multi_ls2/password}@db/sakila",
		},
	)

	require.NoError(t, tr.Exec("config", "secrets", "ls"))
	out := tr.Out.String()
	require.Contains(t, out, "env:DB_PW")
	require.Contains(t, out, "file:/etc/sq/host")
	require.Contains(t, out, "keyring:@multi_ls2/password")
}

// TestCmdConfigSecretsLs_EmptyConfig — no sources means no output and
// no error. Distinguishes "empty list" from "broken command".
func TestCmdConfigSecretsLs_EmptyConfig(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	require.NoError(t, tr.Exec("config", "secrets", "ls"))
	require.Empty(t, tr.Out.String())
}

// TestCmdConfigSecretsLs_HandleAndDriverColumns verifies that each row
// pairs the ref with its source's handle and driver type.
func TestCmdConfigSecretsLs_HandleAndDriverColumns(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Add(
		source.Source{
			Handle:   "@cols_pg",
			Type:     drivertype.Pg,
			Location: "${keyring:abc1234567}",
		},
		source.Source{
			Handle:   "@cols_my",
			Type:     drivertype.MySQL,
			Location: "${env:MY_DSN}",
		},
	)
	require.NoError(t, tr.Exec("config", "secrets", "ls"))
	out := tr.Out.String()

	// One row per (ref, handle); each row carries its driver.
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 2)

	// Find each row by its ref and assert the handle+driver are on the
	// same line. Order is sort-by-ref-then-handle: env:* before keyring:*.
	require.Contains(t, lines[0], "env:MY_DSN")
	require.Contains(t, lines[0], "@cols_my")
	require.Contains(t, lines[0], "mysql")

	require.Contains(t, lines[1], "keyring:abc1234567")
	require.Contains(t, lines[1], "@cols_pg")
	require.Contains(t, lines[1], "postgres")
}

// TestCmdConfigSecretsLs_SharedRefShowsMultipleRows verifies the
// load-bearing Form B property: when two sources reference the same
// keyring ID, the listing makes the sharing visible by emitting one
// row per (ref, source) pair rather than deduplicating.
func TestCmdConfigSecretsLs_SharedRefShowsMultipleRows(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	const sharedID = "r5x2cd9k7w"
	tr.Add(
		source.Source{
			Handle:   "@primary_sh",
			Type:     drivertype.Pg,
			Location: "${keyring:" + sharedID + "}",
		},
		source.Source{
			Handle:   "@replica_sh",
			Type:     drivertype.Pg,
			Location: "${keyring:" + sharedID + "}",
		},
	)

	require.NoError(t, tr.Exec("config", "secrets", "ls"))
	lines := strings.Split(strings.TrimRight(tr.Out.String(), "\n"), "\n")
	require.Len(t, lines, 2, "shared ref should produce one row per source")

	// Both rows carry the same ref; handles distinguish them. Sort
	// order is by ref then handle, so @primary_sh < @replica_sh.
	for _, ln := range lines {
		require.Contains(t, ln, "keyring:"+sharedID)
	}
	require.Contains(t, lines[0], "@primary_sh")
	require.Contains(t, lines[1], "@replica_sh")
}

// TestCmdConfigSecretsLs_CompositionMultipleRowsPerHandle verifies that
// a single source with several embedded placeholders produces one row
// per placeholder.
func TestCmdConfigSecretsLs_CompositionMultipleRowsPerHandle(t *testing.T) {
	gokeyring.MockInit()
	th := testh.New(t)
	tr := testrun.New(th.Context, t, nil)
	tr.Add(source.Source{
		Handle:   "@compo",
		Type:     drivertype.Pg,
		Location: "postgres://${env:DB_USER}:${keyring:abc1234567}@${env:DB_HOST}/sakila",
	})

	require.NoError(t, tr.Exec("config", "secrets", "ls"))
	lines := strings.Split(strings.TrimRight(tr.Out.String(), "\n"), "\n")
	require.Len(t, lines, 3, "composition source should yield one row per placeholder")

	// All three rows reference the same handle; sort order surfaces
	// env:* before keyring:*, and env:DB_HOST before env:DB_USER.
	for _, ln := range lines {
		require.Contains(t, ln, "@compo")
		require.Contains(t, ln, "postgres")
	}
	require.Contains(t, lines[0], "env:DB_HOST")
	require.Contains(t, lines[1], "env:DB_USER")
	require.Contains(t, lines[2], "keyring:abc1234567")
}
