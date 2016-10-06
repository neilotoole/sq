package cmd

import (
	"fmt"
	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/out"
	//_ "github.com/neilotoole/sq/sq/bootstrap"
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
const FlagTableUsage string = "table output"

// FlagHeader specifies that the output should include header information (where applicable)
const FlagHeader string = "header"

// FlagHeaderShort is shorthand for FlagHeader
const FlagHeaderShort string = "h"

// FlagHeaderUsage is usage for FlagHeader
const FlagHeaderUsage string = "show header"

// FlagNoHeader specifies that the output should not include header information (where applicable)
const FlagNoHeader string = "no-header"

// FlagNoHeaderShort is shorthand for FlagHeader
const FlagNoHeaderShort string = "H"

// FlagNoHeaderUsage is usage for FlagNoHeader
const FlagNoHeaderUsage string = "hide header"

// FlagModeNativeSQL specifies SQL mode
const FlagModeNativeSQL string = "native"

// FlagModeNativeSQLShort is shorthand for FlagModeNativeSQL
const FlagModeNativeSQLShort string = "n"

// FlagModeNativeSQLUsage is usage for FlagModeNativeSQL
const FlagModeNativeSQLUsage string = "native SQL mode"

// FlagModeSQ specifies SQ (simple) mode
const FlagModeSQ string = "sq"

// FlagModeSQShort is shorthand for FlagModeSQ
const FlagModeSQShort string = "s"

// FlagModeSQUsage is usage for FlagModeSQ
const FlagModeSQUsage string = "SQ (simple query) mode"

// FlagPingAll indicates to ping all data sources
const FlagPingAll string = "all"

// FlagPingAllShort is shorthand for FlagPingAll
const FlagPingAllShort string = "a"

// FlagPingAllUsage is usage for FlagPingAllShort
const FlagPingAllUsage string = "ping all datasources"

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use: `sq QUERY`,
	Example: `  # Add a data source (run "sq add --help" for more)
  sq add 'mysql://user:pass@localhost:3306/mydb1' @mydb1

  # Set the active datasource
  sq use @mydb1

  # Get schema etc for datasource
  sq inspect @mydb1

  # Table format (with header)
  sq -th '.users | .uid, .username, .email'

  # Native (SQL) syntax:
  sq -n 'select uid, username, email from tbl1'`,
	Short: "sq - simple, uniform access to structured data",
	Long: `sq - the Simple Queryer - provides simple, uniform access to structured data
across many common data sources. Results are output in JSON by default, but can
several output formats are available. sq implements a simple query language,
inspired by the excellent "jq" utility, but traditional database-native SQL
queries can also be used via the --native flag.

For full usage, see the online manual: http://neilotoole.io/sq
`,
	BashCompletionFunction: bash_completion_func,
	//Uncomment the following line if your bare application
	//has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		panic("root.go: should never be executed")
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {

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

	//fmt.Printf("root err: %v", err)
	//fmt.Printf("cmd name: %s\n", cmd.Name())
	//fmt.Printf("cmd args: %s\n", cmdArgs)
	// We determine it's the root command if there's an err returned from
	// Find(), and the cmd.Name is the program name (usually 'sq', but the
	// user could rename it).
	//if (cmdArgs == nil || len(cmdArgs) == 0) || (err != nil && cmd != nil && cmd.Name() == "sq") { // REVISIT: note that when run from debugger, that os.Args[0] can be weird text
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

	//logPath := filepath.Join(util.ConfigDir(), "sq.log")
	//logFile, err := os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	//if err != nil {
	//	fmt.Fprintf(os.Stderr, "Error: unable to access log file: ", err)
	//	os.Exit(1)
	//}
	//lg.Use(logFile)

	cobra.OnInitialize(doInstallBashCompletion)
	cobra.OnInitialize(initWriter)

	// Here you will define your flags and configuration settings.
	// Cobra supports Persistent Flags, which, if defined here,
	// will be global for your application.

	// --json -j JSON (default)
	// --grid -g JSON grid (or --array -a array)
	// --table -t Text table
	// --header -h include header information
	// --type -t include column type information (only if -h is set)
	// --tsv -b tab Separated values
	// --csv -c Command separated values

	//RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.sq.yaml)")
	//RootCmd.PersistentFlags().BoolP("help", "", false, "help for this command")
	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	//RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	preprocessCmd(RootCmd)
	setQueryCmdOptions(RootCmd)
}

//func execRootE(cmd *cobra.Command, args []string) error {
//
//	fmt.Println("You're running sq!")
//	fmt.Printf("Args: %v", strings.Join(args, " "))
//	return nil
//}
//func execRoot(cmd *cobra.Command, args []string) {
//
//	fmt.Println("You're running sq!")
//	fmt.Printf("Args: %v", strings.Join(args, " "))
//	return
//}

func initWriter() {

	////out.Set(json.NewWriter())
	//out.Set(table.NewWriter())
}

// preprocessCmd is should be run on all commands before adding them.
func preprocessCmd(cmd *cobra.Command) {
	cmd.Flags().BoolP("help", "", false, "help for "+cmd.Name())

	if cmd.RunE != nil {
		cmd.RunE = execer(cmd.RunE)
	}

	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

}

// cobraCmdFunc is a function type that matches the Cobra RunE function signature.
type cobraCmdFn func(cmd *cobra.Command, args []string) error

// execer wraps a command's RunE function.
func execer(fn cobraCmdFn) cobraCmdFn {

	return func(cmd *cobra.Command, args []string) error {

		err := fn(cmd, args)

		if err != nil {
			handleError(cmd, err)
		}

		return nil
	}
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
