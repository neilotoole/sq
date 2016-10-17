package cmd

import "github.com/spf13/cobra"

var helpCmd = &cobra.Command{
	Use:    "help",
	RunE:   execHelp,
	Short:  "Display sq help",
	Hidden: true,
}

func init() {
	RootCmd.AddCommand(helpCmd)

}

func execHelp(cmd *cobra.Command, args []string) error {

	return RootCmd.Help()
}
