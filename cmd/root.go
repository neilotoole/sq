package cmd

import (
	"fmt"
	"os"

	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/cmd/config"
	_ "github.com/neilotoole/sq/libsq/drvr/impl"
	"github.com/neilotoole/sq/libsq/shutdown"
	"github.com/neilotoole/sq/libsq/util"
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use: `sq QUERY`,
	Example: `  # get specified cols from tbladdress in active data source
  sq '.tbladdress |  .address_id, .city, .country'

  # register data source
  sq add 'mysql://user:pass@localhost:3306/mydb1' @my1

  # list available data sources
  sq ls

  # set active data source
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
query language (aka "SLQ"), inspired by the excellent "jq" utility, but
traditional database-native SQL queries are also supported.

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

	// We determine it's the root command if there's an err returned from
	// Find(), and the cmd.Name is the program name (usually 'sq', but the
	// user could rename it).
	// TODO: need to check for user renaming the command?
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
	addQueryCmdFlags(RootCmd)
	preprocessCmd(RootCmd)
}

// cfg is the package-level Config instance. By the time the body of a command
// executes, cfg will have been initialized.
var cfg *config.Config

// cfgStore is the package-level config.Store instance. By the time the body of a command
// executes, cfgStore will have been initialized.
var cfgStore *config.FileStore

// initConfig ensures that the package's cfg and store variables are initialized.
func initConfig(cmd *cobra.Command) (*config.Config, config.Store, error) {
	if cfg != nil {
		return cfg, cfgStore, nil
	}

	// cfg isn't loaded yet
	envar := "SQ_CONFIGFILE"
	cfgPath, ok := os.LookupEnv(envar)
	if !ok {
		// envar not set, let's use the user homedir
		dir, err := homedir.Dir()
		if err != nil {
			// really shouldn't happen
			lg.Errorf("failed to get home dir: %v", err)
			lg.Warnf("failing back to current working dir")
			dir, err = os.Getwd()
			if err != nil {
				// also should not happen
				return nil, nil, util.WrapError(err)
			}

		}
		cfgPath = filepath.Join(dir, "sq.yml")
	}

	cfgStore = &config.FileStore{cfgPath}
	lg.Debugf("attempting to use config file %q", cfgStore.Path)
	if !cfgStore.FileExists() {
		lg.Debugf("config file does not exist: %v", cfgStore.Path)
		cfg = config.New()
		return cfg, cfgStore, nil
	}

	// file does exist, let's try to load it
	conf, err := cfgStore.Load()
	if err != nil {
		return nil, nil, err
	}
	lg.Debugf("loaded config file %q", cfgStore.Path)
	cfg = conf
	return cfg, cfgStore, nil
}

// preprocessCmd should be run on all commands before they are added to to cobra.
func preprocessCmd(cmd *cobra.Command) {
	cmd.Flags().BoolP("help", "", false, "Help for "+cmd.Name())

	if cmd.RunE != nil {
		// Wrap the cmd run function in the decorator below, enabling
		// centralized error handling and what not.
		execFn := cmd.RunE
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			lg.Debugf("invoking cmd %q: %v", cmd.Name(), args)
			err := execFn(cmd, args)
			if err != nil {
				handleError(cmd, err)
			}
			return nil
		}
	}

	if cmd.PreRunE == nil {
		cmd.PreRunE = preExec
	}

	// We handle the errors ourselves (rather than let cobra do it)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
}

// preExec is called before a cmd is executed.
func preExec(cmd *cobra.Command, args []string) error {

	lg.Debugf("executing command %q: %v", cmd.Name(), args)
	_, _, err := initConfig(cmd)
	if err != nil {
		return err
	}

	initWriter(cmd, cfg)
	return nil
}

// handleError is the centralized function for handling CLI errors. It prints
// the error, invokes the shutdown mechanism, and ultimately invokes os.Exit(1).
func handleError(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}

	cmdName := ""
	if cmd != nil {
		cmdName = fmt.Sprintf("[cmd:%s] ", cmd.Name())
	}

	lg.Depth(1).Errorf(fmt.Sprintf("%s%v", cmdName, err))

	if wrtr == nil {
		initWriter(cmd, cfg)
	}

	wrtr.Error(err)
	shutdown.Shutdown(1)
}
