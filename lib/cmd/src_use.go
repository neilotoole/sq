package cmd

import (
	"fmt"

	"github.com/neilotoole/sq/lib/out/table"
	"github.com/spf13/cobra"
)

var srcCmd = &cobra.Command{
	Use: "src [@HANDLE]",
	Example: `  # get active data source
   sq src
   # set @my1 as active data source
   sq src @my1`,
	RunE:    execUse,
	Short:   "Get or set active data source",
	Aliases: []string{"using"},
	Long: `Get or set active data source. If no arguments provided, get the active data
source. Otherwise, set @HANDLE as the active data source.`,
}

func init() {
	preprocessCmd(srcCmd)
	RootCmd.AddCommand(srcCmd)

}

func execUse(cmd *cobra.Command, args []string) error {

	if len(args) > 1 {
		return fmt.Errorf("invalid arguments")
	}

	w := table.NewWriter(true)
	//cfg := cfg

	if len(args) == 0 {
		// Get the active data source
		src, ok := cfg.SourceSet.Active()
		if !ok {
			return nil
		}

		w.Source(src)
		return nil

	}

	src, err := cfg.SourceSet.SetActive(args[0])
	if err != nil {
		return err
	}

	err = cfg.Save()
	if err != nil {
		return err
	}

	w.Source(src)
	return nil
}
