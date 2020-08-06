package cli

import "github.com/spf13/cobra"

func newHelpCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:    "help",
		Short:  "Show sq help",
		Hidden: true,
	}

	return cmd, execHelp
}

func execHelp(rc *RunContext, cmd *cobra.Command, args []string) error {
	return cmd.Root().Help()
}
