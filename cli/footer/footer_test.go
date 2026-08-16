package footer

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func TestInteractiveSession_nonTerminal(t *testing.T) {
	t.Parallel()

	ru := &run.Run{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}
	require.False(t, interactiveSession(ru, nil))
}

func TestInteractiveSession_commandError(t *testing.T) {
	t.Parallel()

	ru := &run.Run{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	require.False(t, interactiveSession(ru, assertError{}))
}

func TestFooterSuppressingError(t *testing.T) {
	t.Parallel()

	require.False(t, footerSuppressingError(nil))
	require.False(t, footerSuppressingError(errz.ErrNoQuery))
	require.False(t, footerSuppressingError(assertError{msg: "accepts 1 arg(s), received 0"}))
	require.True(t, footerSuppressingError(assertError{}))
}

func TestInteractiveSession_envOptOut(t *testing.T) {
	t.Setenv(envNoUpdateNotifier, "1")
	ru := &run.Run{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	require.False(t, interactiveSession(ru, nil))
}

func TestInteractiveSession_fileOutput(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{Use: "slq"}
	cmd.Flags().String(flag.FileOutput, "", "")
	require.NoError(t, cmd.Flags().Set(flag.FileOutput, "out.csv"))
	cmd.Flags().Lookup(flag.FileOutput).Changed = true

	ru := &run.Run{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Cmd:    cmd,
	}
	require.False(t, interactiveSession(ru, nil))
}

func TestRender_skipsWhenNotInteractive(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	ru := &run.Run{
		Stdout: buf,
		Stderr: buf,
	}
	Render(context.Background(), ru, nil)
	require.Empty(t, buf.String())
}

type assertError struct{ msg string }

func (e assertError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return "boom"
}
