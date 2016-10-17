package cmd

import "github.com/spf13/cobra"

var srcListCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List data sources",
	RunE:    execSrcList,
}

func init() {
	preprocessCmd(srcListCmd)
	RootCmd.AddCommand(srcListCmd)
}

func execSrcList(cmd *cobra.Command, args []string) error {
	if cfg.SourceSet == nil {
		return nil
	}

	w := getWriter(cmd)
	active, _ := cfg.SourceSet.Active()

	if len(cfg.SourceSet.Items) > 0 {
		w.SourceSet(cfg.SourceSet, active)
	}

	return nil
}
