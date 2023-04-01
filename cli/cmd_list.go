package cli

import (
	"github.com/spf13/cobra"
)

func newSrcListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List data sources",
		Long:  "List data sources.",
		Args:  cobra.ExactArgs(0),
		RunE:  execSrcList,
	}

	cmd.Flags().BoolP(flagHeader, flagHeaderShort, false, flagHeaderUsage)
	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	return cmd
}

func execSrcList(cmd *cobra.Command, _ []string) error {
	rc := RunContextFrom(cmd.Context())

	return rc.writers.srcw.SourceSet(rc.Config.Sources)
}
