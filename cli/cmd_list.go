package cli

import (
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/libsq/errz"
)

func newSrcListCmd() (*cobra.Command, runFunc) {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List data sources",
	}

	cmd.Flags().BoolP(flagVerbose, flagVerboseShort, false, flagVerboseUsage)
	cmd.Flags().BoolP(flagHeader, flagHeaderShort, false, flagHeaderUsage)
	return cmd, execSrcList
}

func execSrcList(rc *RunContext, cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		return errz.Errorf(msgInvalidArgs)
	}

	return rc.writers.srcw.SourceSet(rc.Config.Sources)
}
