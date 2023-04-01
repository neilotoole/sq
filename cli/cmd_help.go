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

func execHelp(cmd *cobra.Command, _ []string) error {
	return cmd.Root().Help()
}
