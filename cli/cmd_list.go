package cli

import (
	"github.com/spf13/cobra"
)

func newSrcListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List data sources",
		Args:  cobra.ExactArgs(0),
		RunE:  execSrcList,
	}

	cmd.Flags().BoolP(flagVerbose, flagVerboseShort, false, flagVerboseUsage)
	cmd.Flags().BoolP(flagHeader, flagHeaderShort, false, flagHeaderUsage)
	return cmd
}

func execSrcList(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())

	return rc.writers.srcw.SourceSet(rc.Config.Sources)
}
