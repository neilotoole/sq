package cli

import (
	"github.com/neilotoole/sq/cli/flag"
	"github.com/spf13/cobra"
)

func newDriverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "driver",
		Short: "Manage drivers",
		Long:  "Manage drivers.",
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
		Short: "List installed drivers",
		Long:  "List installed drivers.",
		Args:  cobra.NoArgs,
		RunE:  execDriverList,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)

	return cmd
}

func execDriverList(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())
	drvrs := rc.driverReg.DriversMetadata()
	return rc.writers.Metadata.DriverMetadata(drvrs)
}
