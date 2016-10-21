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
	cfg, _, w, err := ioFor(cmd, args)
	if err != nil {
		return err
	}
	if cfg.SourceSet == nil {
		return nil
	}

	active, _ := cfg.SourceSet.Active()
	return w.SourceSet(cfg.SourceSet, active)
}
