package cli

import (
	"github.com/neilotoole/sq/cli/flag"
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
		Short: "List available drivers.",
		Args:  cobra.ExactArgs(0),
		RunE:  execDriverList,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Table, flag.TableShort, false, flag.TableUsage)
	cmd.Flags().BoolP(flag.Header, flag.HeaderShort, false, flag.HeaderUsage)

	return cmd
}

func execDriverList(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())
	drvrs := rc.registry.DriversMetadata()
	return rc.writers.metaw.DriverMetadata(drvrs)
}
