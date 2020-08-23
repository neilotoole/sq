package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/core/errz"
)

func newDriversCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:   "drivers",
		Short: "List available drivers",
	}

	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	cmd.Flags().BoolP(flagTable, flagTableShort, false, flagTableUsage)
	cmd.Flags().BoolP(flagHeader, flagHeaderShort, false, flagHeaderUsage)
	cmd.Flags().BoolP(flagMonochrome, flagMonochromeShort, false, flagMonochromeUsage)

	return cmd, execDrivers
}

func execDrivers(rc *RunContext, cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errz.Errorf("invalid arguments: zero arguments expected")
	}

	drvrs := rc.registry.DriversMetadata()
	return rc.writers.metaw.DriverMetadata(drvrs)
}
