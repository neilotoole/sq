package cmd

import (
	"fmt"

	"github.com/neilotoole/sq/lib/config"
	"github.com/neilotoole/sq/lib/out/table"
	"github.com/spf13/cobra"
)

var srcUse = &cobra.Command{
	Use:     "use [DATASOURCE]",
	RunE:    execUse,
	Short:   "Set or get the active data source",
	Aliases: []string{"use"},
	Long: `Set or get the active data source.
If no arguments provided, get the active data source. Otherwise, set DATASOURCE
as the active data source.

Examples:

   # Get the active data source
   sq use
   # Set "mydb1" to be active data source
   sq use mydb1

More:

Use "sq ls" to list available data sources, or "sq add" to add a new
data source.
`,
}

func init() {
	preprocessCmd(srcUse)
	RootCmd.AddCommand(srcUse)

}

func execUse(cmd *cobra.Command, args []string) error {

	if len(args) > 1 {
		return fmt.Errorf("invalid arguments")
	}

	w := table.NewWriter(true)
	cfg := config.Default()

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
