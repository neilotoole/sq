package cmd

import (
	"fmt"

	"github.com/neilotoole/sq/lib/config"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/util"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use: "inspect [@HANDLE]",
	Example: `  # inspect active data source
  sq inspect

  # inspect @pg1 and output in table format
  sq inspect --th @pg1`,
	Short: "Inspect data source schema and stats",
	Long: `Inspect a data source, including table schemata, columns, etc.
If @HANDLE is not provided, use the active data source.`,
	Aliases: []string{"info"},
	RunE:    inspect,
}

func init() {
	preprocessCmd(inspectCmd)
	setQueryOutputOptions(inspectCmd)
	RootCmd.AddCommand(inspectCmd)

}

func inspect(cmd *cobra.Command, args []string) error {

	if len(args) > 1 {
		return util.Errorf("invalid arguments")
	}

	var err error
	var src *driver.Source
	if len(args) == 0 {
		ok := false
		src, ok = config.Default().SourceSet.Active()
		if !ok {
			return fmt.Errorf("can't get active datasource")
		}
	} else {

		src, err = config.Default().SourceSet.Get(args[0])
		if err != nil {
			return err
		}
	}

	drv, err := driver.For(src)
	if err != nil {
		return err
	}

	meta, err := drv.Metadata(src)
	if err != nil {
		return err
	}

	return getResultWriter(cmd).Metadata(meta)
}
