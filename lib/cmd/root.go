package cmd

import (
	"fmt"
	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/out"
	//_ "github.com/neilotoole/sq/sq/bootstrap"
	"path/filepath"

	"sync"

	"github.com/mitchellh/go-homedir"
	"github.com/neilotoole/sq/lib/config"
	"github.com/neilotoole/sq/lib/shutdown"
	"github.com/spf13/cobra"
)

// FlagJSON specifies JSON output
const FlagJSON string = "json"

// FlagJSONShort is shorthand for FlagJSON
const FlagJSONShort string = "j"

// FlagJSONUsage is usage for FlagJSON
const FlagJSONUsage string = "JSON output"

// FlagRaw specifies raw output
const FlagRaw string = "raw"

// FlagRawShort is shorthand for FlagRaw
const FlagRawShort string = "r"

// FlagRawUsage is usage for FlagRaw
const FlagRawUsage string = "Output each result directly to stdout"

// FlagTable specifies Table output
const FlagTable string = "table"

// FlagTableShort is shorthand for FlagTable
const FlagTableShort string = "t"

// FlagTableUsage is usage for FlagTable
const FlagTableUsage string = "Table output"

// FlagXLSX specifies XLSX output
const FlagXLSX string = "xlsx"

// FlagXLSXShort is shorthand for FlagXLSX
const FlagXLSXShort string = "x"

// FlagXLSXUsage is usage for FlagXLSX
const FlagXLSXUsage string = "XLSX (Excel) output"

// FlagCSV specifies CSV output
const FlagCSV string = "csv"

// FlagCSVShort is shorthand for FlagCSV
const FlagCSVShort string = "c"

// FlagCSVUsage is usage for FlagCSV
const FlagCSVUsage string = "CSV (comma-separated value) output"

// FlagTSV specifies TSV output
const FlagTSV string = "tsv"

// FlagTSVShort is shorthand for FlagTSV
const FlagTSVShort string = "T"

// FlagTSVUsage is usage for FlagTSV
const FlagTSVUsage string = "TSV (tab-separated value) output"

// FlagXML specifies XML output
const FlagXML string = "xml"

// FlagXMLShort is shorthand for FlagXML
const FlagXMLShort string = "X"

// FlagXMLUsage is usage for FlagXML
const FlagXMLUsage string = "XML output"

// FlagMonochrome specifies raw output
const FlagMonochrome string = "monochrome"

// FlagMonochromeShort is shorthand for FlagMonochrome
const FlagMonochromeShort string = "M"

// FlagMonochromeUsage is usage for FlagMonochrome
const FlagMonochromeUsage string = "don't colorize output"

// FlagHeader specifies that the output should include header information (where applicable)
const FlagHeader string = "header"

// FlagHeaderShort is shorthand for FlagHeader
const FlagHeaderShort string = "h"

// FlagHeaderUsage is usage for FlagHeader
const FlagHeaderUsage string = "Print header"

// FlagNoHeader specifies that the output should not include header information (where applicable)
const FlagNoHeader string = "no-header"

// FlagNoHeaderShort is shorthand for FlagHeader
const FlagNoHeaderShort string = "H"

// FlagNoHeaderUsage is usage for FlagNoHeader
const FlagNoHeaderUsage string = "Do not print header"

// FlagModeNativeSQL specifies SQL mode
const FlagModeNativeSQL string = "native"

// FlagModeNativeSQLShort is shorthand for FlagModeNativeSQL
const FlagModeNativeSQLShort string = "n"

// FlagModeNativeSQLUsage is usage for FlagModeNativeSQL
const FlagModeNativeSQLUsage string = "Native SQL query mode"

// FlagModeSQ specifies SQ (simple) mode
const FlagModeSQ string = "sq"

// FlagModeSQShort is shorthand for FlagModeSQ
const FlagModeSQShort string = "s"

// FlagModeSQUsage is usage for FlagModeSQ
const FlagModeSQUsage string = "SQ query mode"

// FlagPingAll indicates to ping all data sources
const FlagPingAll string = "all"

// FlagPingAllShort is shorthand for FlagPingAll
const FlagPingAllShort string = "a"

// FlagPingAllUsage is usage for FlagPingAllShort
const FlagPingAllUsage string = "Ping all datasources"

const FlagDriver string = "driver"

const FlagDriverUsage string = "Explicitly specify the data source driver to use"

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use: `sq QUERY`,
	Example: `  # get specified cols from tbladdress in active datasource
  sq '.tbladdress |  .address_id, .city, .country'

  # add a new datasource
  sq add 'mysql://user:pass@localhost:3306/mydb1' @my1

  # list the available data sources
  sq ls

  # set the active data source
  sq src @my1

  # get schema etc for data source
  sq inspect @my1

  # output in table format (with header)
  sq -th '.user | .uid, .username, .email'

  # join across data sources
  sq '@my1.user, @pg1.tbladdress | join(.uid) | .username, .email, .city'

  # native (SQL) query:
  sq -n 'SELECT uid, username, email FROM user'`,
	Short: "sq - simple queryer for structured data",
	Long: `sq - simple queryer for structured data - provides simple, uniform access to
structured data across many common data sources. Results are output in JSON by
default, but several output formats are available. sq uses a simple universal
query language, inspired by the excellent "jq" utility, but traditional
database-native SQL queries are also available.

For full usage, see the online manual: http://neilotoole.io/sq
`,
	BashCompletionFunction: bash_completion_func,
	//Uncomment the following line if your bare application
	//has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		panic("root.go: should never be executed")
	},
}

var cfg *config.Config
var cfgStore config.Store

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {

	err := initConfig()
	if err != nil {
		handleError(nil, err)
		return
	}

	// HACK: This is a workaround for the fact that cobra doesn't currently
	// support executing the root command with arbitrary args. That is to say,
	// if you execute:
	//
	//   sq arg1 arg2
	//
	// then cobra will look for a command named "arg1", and when it
	// doesn't find such a command, it returns an "unknown command"
	// error.
	// There are currently a number of open PRs for this issue, but
	// none have been merged into cobra's master yet (2016/07/28)

	cmd, _, _ := RootCmd.Find(os.Args[1:])

	// We determine it's the root command if there's an err returned from
	// Find(), and the cmd.Name is the program name (usually 'sq', but the
	// user could rename it).
	//if (cmdArgs == nil || len(cmdArgs) == 0) || (err != nil && cmd != nil && cmd.Name() == "sq") {
	if cmd != nil && cmd.Name() == "sq" { // REVISIT: note that when run from debugger, that os.Args[0] can be weird text

		// Basically, if we have os.Args: [sq, arg1, arg2]
		// We redirect to the "query" command by changing os.Args to
		// look like: [sq, query, arg1, arg2]
		hackArgs := []string{os.Args[0], "query"}
		hackArgs = append(hackArgs, os.Args[1:]...)
		os.Args = hackArgs
	}

	if err := RootCmd.Execute(); err != nil {

		cmd, _, _ := RootCmd.Find(os.Args[1:])
		handleError(cmd, err)
	}

	shutdown.Shutdown(0)
}

func init() {

	cobra.OnInitialize(doInstallBashCompletion)
	preprocessCmd(RootCmd)
	setQueryCmdOptions(RootCmd)
}

var cfgMu sync.Mutex

func getConfig() *config.Config {

	cfgMu.Lock()
	defer cfgMu.Unlock()

	if cfg == nil {
		cfg = config.NewConfig()
	}

	return cfg
}

func initConfig() error {

	envar := "SQ_CONFIG_FILEPATH"
	configPath, ok := os.LookupEnv(envar)

	if !ok {
		configPath = filepath.Join(configDir(), "sq.yml")
	}

	lg.Debugf("attempting to create filestore from %q with value %q", envar, configPath)
	var err error
	cfgStore, err = config.NewFileStore(configPath)
	if err != nil {
		return err
	}

	cfg, err = cfgStore.Load()
	if err != nil {
		return err
	}

	return nil
}

func saveConfig() error {

	return cfgStore.Save(cfg)
}

// configDir returns the absolute path of "~/.sq/" (or an alternative if specified by the user)
func configDir() string {

	home, err := homedir.Dir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to get user homedir: %v", err)
		os.Exit(1)
	}

	return filepath.Join(home, ".sq")
}

// preprocessCmd should be run on all commands before adding them.
func preprocessCmd(cmd *cobra.Command) {
	cmd.Flags().BoolP("help", "", false, "help for "+cmd.Name())

	if cmd.RunE != nil {
		cmd.RunE = execer(cmd.RunE)
	}

	if cmd.PreRunE == nil {
		cmd.PreRunE = preExec
	}

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

}

// cobraCmdFunc is a function type that matches the Cobra RunE function signature.
type cobraCmdFn func(cmd *cobra.Command, args []string) error

// execer wraps a command's RunE function.
func execer(fn cobraCmdFn) cobraCmdFn {

	return func(cmd *cobra.Command, args []string) error {

		lg.Debugf("invoking cmd %q: %v", cmd.Name(), args)

		err := fn(cmd, args)

		if err != nil {
			handleError(cmd, err)
		}

		return nil
	}
}

// preExec is called before a cmd is executed.
func preExec(cmd *cobra.Command, args []string) error {

	lg.Debugf("preExec cmd %q: %v", cmd.Name(), args)

	return nil
}

func handleError(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}

	cmdName := ""
	if cmd != nil {
		cmdName = fmt.Sprintf("[cmd:%s] ", cmd.Name())
	}

	lg.Depth(1).Errorf(fmt.Sprintf("%s%v", cmdName, err))
	fmt.Fprintf(os.Stderr, out.Trans.Error("Error:")+" %v\n", err)
	shutdown.Shutdown(1)

}
