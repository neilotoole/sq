package cmd

import (
	"github.com/neilotoole/sq/libsq/util"
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

	if len(args) != 0 {
		return util.Errorf("invalid arguments")
	}

	//cfg, _, w, err := ioFor(cmd, args)
	//if err != nil {
	//	return err
	//}
	//if cfg.SourceSet == nil {
	//	return nil
	//}

	active, _ := cfg.SourceSet.Active()
	return wrtr.SourceSet(cfg.SourceSet, active)
}
