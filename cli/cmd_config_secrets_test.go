package cli_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	gokeyring "github.com/zalando/go-keyring"

	"github.com/neilotoole/sq/cli/testrun"
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
