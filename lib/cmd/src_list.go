package cmd

import (
	"github.com/neilotoole/sq/lib/out/table"
	"github.com/spf13/cobra"
)

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

	//cfg := cfg

	w := table.NewWriter(true)

	active, _ := cfg.SourceSet.Active()

	if len(cfg.SourceSet.Items) > 0 {
		w.SourceSet(cfg.SourceSet, active)
	}

	return nil
}
