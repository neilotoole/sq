package cmd

import (
	"fmt"

	"github.com/neilotoole/sq/lib/config"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/util"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use: "inspect [DATASOURCE]",
	Example: `  sq inspect
  sq inspect @pg1 --th`,
	Short: "Inspect a datasource",
	Long: `Inspect a datasource, including table schemata, columns, etc.
If DATASOURCE is not provided, use the active datasource.`,
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
