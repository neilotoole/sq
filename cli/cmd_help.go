package cli

import "github.com/spf13/cobra"

func newHelpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "help",
		Short:  "Show sq help",
		Hidden: true,
		RunE:   execHelp,
	}

	return cmd
}

func execHelp(cmd *cobra.Command, args []string) error {
	return cmd.Root().Help()
}
