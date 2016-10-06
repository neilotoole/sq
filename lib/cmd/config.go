package cmd

import (
	"fmt"

	"strconv"

	"github.com/neilotoole/sq/lib/config"
	"github.com/neilotoole/sq/lib/out/table"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var configCmd = &cobra.Command{
	Use:     "cfg",
	Aliases: []string{"config"},
	Short:   "Configure or list default options",
	Hidden:  true, // TODO (neilotoole): get rid of this command? (user should just edit config file?)
	RunE:    execConfig,
	Long: `Configure or list default options. When executing a query, these defaults can
be overridden by command line flags.

List defaults:

    sq cfg

Set defaults:
Use the same flags available on the root sq command; these choices will be
remembered (but can be overwritten by command line flags).

    sq cfg --FLAGS

Examples:
   # List defaults
   sq cfg

   # Set defaults
   sq cfg --sql --table --header     # mode is "sql", format is "table", show header
   sq cfg -lth                       # same as above, using flag shorthand
   sq cfg --sq --grid --no-header    # mode is "sq", format is "grid", hide header
   sq cfg -sgH                       # same as above, using shorthand
`,
}

func init() {
	preprocessCmd(configCmd)
	setQueryCmdOptions(configCmd)
	RootCmd.AddCommand(configCmd)
}

func execConfig(cmd *cobra.Command, args []string) error {

	cfg := config.Default()
	w := table.NewWriter(false)

	if len(args) > 0 {
		return fmt.Errorf("invalid arguments")
	}

	if !hasChangedFlags(cmd) {
		//var rows [][]string = make([][]string, 3)
		//rows[0] = []string{"mode", string(cfg.Options.QueryMode)}
		//rows[1] = []string{"format", string(cfg.Options.Format)}
		//rows[2] = []string{"headers", strconv.FormatBool(cfg.Options.Headers)}
		//w.Rows(rows, []tablewriter.Transformer{out.FgBlueTransform})
		doPrintCfgDefaults(cfg, w)
		return nil
	}

	if cmd.Flags().Changed(FlagModeNativeSQL) {
		cfg.Options.QueryMode = config.ModeNativeSQL
	}

	if cmd.Flags().Changed(FlagModeSQ) {
		cfg.Options.QueryMode = config.ModeSQ
	}

	if cmd.Flags().Changed(FlagTable) {
		cfg.Options.Format = config.FormatTable
	}
	if cmd.Flags().Changed(FlagJSON) {
		cfg.Options.Format = config.FormatJSON
	}

	if cmd.Flags().Changed(FlagHeader) {
		cfg.Options.Header = true
	}
	if cmd.Flags().Changed(FlagNoHeader) {
		cfg.Options.Header = false
	}

	err := cfg.Save()
	if err != nil {
		return err
	}

	doPrintCfgDefaults(cfg, w)
	return nil
}

/*
	var row []string

	switch args[0] {
	case "mode":
		m := config.QueryMode(args[1])
		switch m {
		case config.ModeSQ, config.ModeSQL:
			cfg.Options.QueryMode = m
			row = []string{"mode", string(m)}
		default:
			return fmt.Errorf("invalid mode %q", m)
		}
	case "format":
		f := config.Format(args[1])
		switch f {
		case config.FormatJSON, config.FormatTable:
			cfg.Options.Format = f
			row = []string{"format", string(f)}
		case config.FormatGrid:
			return fmt.Errorf("format %q not yet implemented", f)
		default:
			return fmt.Errorf("invalid format %q", f)
		}
	case "headers":
		b, err := strconv.ParseBool(args[1])
		if err != nil {
			return fmt.Errorf(`value for "headers" must be one of: true|false`)
		}
		cfg.Options.Headers = b
		row = []string{"headers", strconv.FormatBool(b)}
	default:
		return fmt.Errorf("unknown config option %q", args[0])
	}

	err := cfg.Save()
	if err != nil {
		return err
	}
	w.Rows([][]string{row}, []tablewriter.Transformer{out.FgBlueTransform})
	return nil
*/

//}

// hasChangedFlags returns true if at least one of the cmd's flags has changed.
func hasChangedFlags(cmd *cobra.Command) bool {

	//flags := cmd.Flags()

	changed := false
	cmd.Flags().Visit(func(f *pflag.Flag) {
		//fmt.Printf("flag: %s(%s)\n", f.Name, f.Value.String())
		changed = true
	})

	return changed

}

func doPrintCfgDefaults(cfg *config.Config, w *table.TextWriter) {
	var rows [][]string = make([][]string, 3)
	rows[0] = []string{"mode", string(cfg.Options.QueryMode)}
	rows[1] = []string{"format", string(cfg.Options.Format)}
	rows[2] = []string{"header", strconv.FormatBool(cfg.Options.Header)}
	w.Rows(rows, nil)
}
