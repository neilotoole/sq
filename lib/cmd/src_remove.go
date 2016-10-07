package cmd

import (
	"fmt"

	"github.com/neilotoole/sq/lib/config"
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
		return fmt.Errorf("invalid arguments")
	}

	cfg := config.Default()

	src, err := cfg.SourceSet.Get(args[0])
	if err != nil {
		return err
	}

	err = cfg.SourceSet.Remove(src.Ref)
	if err != nil {
		return err
	}

	err = cfg.Save()
	if err != nil {
		return err
	}

	fmt.Printf("Datasource removed: %q", src.Ref)
	//db := db.New(src)
	//
	//err = db.New(src).Ping()
	//if err != nil {
	//	return err
	//}
	//
	//color.Set(color.FgGreen, color.Bold)
	//fmt.Printf("pong!")
	//color.Unset()
	//fmt.Printf("\n")

	return nil
}
