package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestCmdMarkReadOnly(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	require.False(t, cmdIsReadOnly(cmd), "unmarked cmd must not be read-only")

	cmdMarkReadOnly(cmd)
	require.True(t, cmdIsReadOnly(cmd))

	require.False(t, cmdIsReadOnly(nil), "nil cmd must not panic")
	require.False(t, cmdIsReadOnly(&cobra.Command{Use: "other"}),
		"marking one cmd must not affect another")
}

// TestReadOnlyCmdsAreMarked verifies that the commands that are
// read-only by definition carry the cmdMarkReadOnly annotation, which
// preRun consumes to apply driver.WithReadOnly centrally.
func TestReadOnlyCmdsAreMarked(t *testing.T) {
	testCases := []struct {
		name string
		cmd  *cobra.Command
	}{
		{"inspect", newInspectCmd()},
		{"diff", newDiffCmd()},
		{"ping", newPingCmd()},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, cmdIsReadOnly(tc.cmd),
				"command %q must be marked read-only", tc.name)
		})
	}
}
