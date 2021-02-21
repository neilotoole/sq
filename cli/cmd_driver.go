package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/core/errz"
)

func newDriverCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:   "driver",
		Short: "List or manage drivers",

		Example: `  # List drivers
  $ sq driver ls

  # Install User Driver [TBD]
  $ sq driver install ./rss.sq.yml
`,
	}

	return cmd, func(rc *RunContext, cmd *cobra.Command, args []string) error {
		return cmd.Help()
	}
}

func newDriverListCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List available drivers",
	}

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	cmd.Flags().BoolP(flagTable, flagTableShort, false, flagTableUsage)
	cmd.Flags().BoolP(flagHeader, flagHeaderShort, false, flagHeaderUsage)
	cmd.Flags().BoolP(flagMonochrome, flagMonochromeShort, false, flagMonochromeUsage)

	return cmd, execDriverList
}

func execDriverList(rc *RunContext, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errz.Errorf("invalid arguments: zero arguments expected")
	}

	drvrs := rc.registry.DriversMetadata()
	return rc.writers.metaw.DriverMetadata(drvrs)
}
