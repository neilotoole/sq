package cli

import (
	"github.com/spf13/cobra"
)

func newDriverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "driver",
		Short: "List or manage drivers",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},

		Example: `  # List drivers
  $ sq driver ls`,
	}

	return cmd
}

func newDriverListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List available drivers",
		Args:  cobra.ExactArgs(0),
		RunE:  execDriverList,
	}

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	cmd.Flags().BoolP(flagTable, flagTableShort, false, flagTableUsage)
	cmd.Flags().BoolP(flagHeader, flagHeaderShort, false, flagHeaderUsage)

	return cmd
}

func execDriverList(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	drvrs := rc.registry.DriversMetadata()
	return rc.writers.metaw.DriverMetadata(drvrs)
}
