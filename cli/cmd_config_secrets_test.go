package cli_test

import (
	"os"
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
		name           string
		inLocation     string
		wantLocation   string
		wantKeyring    string // empty => no keyring write expected
		wantSkipReason string // substring expected in stdout for skipped sources
	}{
		{
			name:         "url with password",
			inLocation:   "postgres://alice:hunter2@db/sakila",
			wantLocation: "postgres://alice:${keyring:@h/password}@db/sakila",
			wantKeyring:  "hunter2",
		},
		{
			name:           "url without password",
			inLocation:     "postgres://alice@db/sakila",
			wantLocation:   "postgres://alice@db/sakila",
			wantSkipReason: "no password",
		},
		{
			name:           "non-url",
			inLocation:     "/data/file.xlsx",
			wantLocation:   "/data/file.xlsx",
			wantSkipReason: "not a URL",
		},
		{
			name:           "already templated",
			inLocation:     "postgres://alice:${keyring:@h/password}@db/sakila",
			wantLocation:   "postgres://alice:${keyring:@h/password}@db/sakila",
			wantSkipReason: "already",
		},
		{
			name:         "url-encoded password is decoded into keyring",
			inLocation:   "postgres://alice:p%40ss%3Aword@db/sakila",
			wantLocation: "postgres://alice:${keyring:@h/password}@db/sakila",
			wantKeyring:  "p@ss:word",
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
			require.Equal(t, tc.wantLocation, src.Location)

			if tc.wantKeyring != "" {
				got, err := gokeyring.Get("sq", "@h/password")
				require.NoError(t, err)
				require.Equal(t, tc.wantKeyring, got)
			} else {
				_, err := gokeyring.Get("sq", "@h/password")
				require.ErrorIs(t, err, gokeyring.ErrNotFound)
			}

			if tc.wantSkipReason != "" {
				require.Contains(t, tr.Out.String(), tc.wantSkipReason)
			}
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
	// Keyring unchanged.
	_, err := gokeyring.Get("sq", "@h_dr/password")
	require.ErrorIs(t, err, gokeyring.ErrNotFound)
	// Output describes the planned change.
	require.Contains(t, tr.Out.String(), "@h_dr")
	require.Contains(t, tr.Out.String(), "${keyring:@h_dr/password}")
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
	require.Contains(t, out, "@sakila_ls/password")
	require.Contains(t, out, "@prod_pg_ls/dsn")
	require.NotContains(t, out, "@plain_ls")
}
