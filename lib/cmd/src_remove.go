package cmd

import (
	"fmt"

	"github.com/neilotoole/sq/lib/util"
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

	if len(args) != 1 {
		return util.Errorf("invalid arguments")
	}

	//cfg := cfg

	src, err := cfg.SourceSet.Get(args[0])
	if err != nil {
		return err
	}

	err = cfg.SourceSet.Remove(src.Handle)
	if err != nil {
		return err
	}

	err = saveConfig()
	if err != nil {
		return err
	}

	fmt.Printf("Removed data source %s\n", src.Handle)

	return nil
}
