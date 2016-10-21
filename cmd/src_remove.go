package cmd

import (
	"fmt"

	"github.com/neilotoole/sq/cmd/out"
	"github.com/neilotoole/sq/libsq/util"
	"github.com/spf13/cobra"
)

// srcRemoveCmd represents the remove command
var srcRemoveCmd = &cobra.Command{
	Use:     "rm @HANDLE",
	Example: `  sq rm @my1`,
	Aliases: []string{"remove"},
	Short:   "Remove data source",
	RunE:    execSrcRemove,
}

func init() {
	preprocessCmd(srcRemoveCmd)
	RootCmd.AddCommand(srcRemoveCmd)

}

func execSrcRemove(cmd *cobra.Command, args []string) error {

	cfg, store, _, err := ioFor(cmd, args)
	if err != nil {
		return err
	}

	if len(args) != 1 {
		return util.Errorf("invalid arguments")
	}

	src, err := cfg.SourceSet.Get(args[0])
	if err != nil {
		return err
	}

	err = cfg.SourceSet.Remove(src.Handle)
	if err != nil {
		return err
	}

	err = store.Save(cfg)
	if err != nil {
		return err
	}

	// TODO: move to using getWriter() mechanism
	fmt.Printf("Removed data source ")
	out.Color.Hilite.Printf("%s", src.Handle)
	fmt.Println()

	return nil
}
