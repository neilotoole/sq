package cmd

import (
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/util"
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
	inspectCmd.Flags().BoolP(FlagJSON, FlagJSONShort, false, FlagJSONUsage)
	inspectCmd.Flags().BoolP(FlagTable, FlagTableShort, false, FlagTableUsage)
	inspectCmd.Flags().BoolP(FlagHeader, FlagHeaderShort, false, FlagHeaderUsage)
	inspectCmd.Flags().BoolP(FlagNoHeader, FlagNoHeaderShort, false, FlagNoHeaderUsage)
	inspectCmd.Flags().BoolP(FlagMonochrome, FlagMonochromeShort, false, FlagMonochromeUsage)
	RootCmd.AddCommand(inspectCmd)
}

func inspect(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return util.Errorf("invalid arguments")
	}

	var src *drvr.Source
	if len(args) == 0 {
		ok := false
		src, ok = cfg.SourceSet.Active()
		if !ok {
			return util.Errorf("no data source specified and no active data source")
		}
	} else {

		var err error
		src, err = cfg.SourceSet.Get(args[0])
		if err != nil {
			return err
		}
	}

	drv, err := drvr.For(src)
	if err != nil {
		return err
	}

	meta, err := drv.Metadata(src)
	if err != nil {
		return err
	}

	return wrtr.Metadata(meta)
}
