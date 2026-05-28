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
