// Package cli implements sq's CLI. The spf13/cobra library
// is used, with some notable modifications.
//
// Although cobra provides excellent functionality, it has some issues.
// Most prominently, its documentation suggests reliance
// upon package-level constructs for initializing the
// command tree (bad for testing).
//
// Thus, this cmd package deviates from cobra's suggested
// usage pattern by eliminating all pkg-level constructs
// (which makes testing easier).
//
// The entry point to this pkg is the Execute function.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/neilotoole/sq/cli/config/options"

	"github.com/neilotoole/sq/cli/config/options/format"

	"github.com/neilotoole/sq/cli/flag"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/cli/buildinfo"

	"golang.org/x/exp/slog"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/neilotoole/sq/cli/config"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
)

func init() { //nolint:gochecknoinits
	cobra.EnableCommandSorting = false
}

// errNoMsg is a sentinel error indicating that a command
// has failed, but that no error message should be printed.
// This is useful in the case where any error information may
// already have been printed as part of the command output.
var errNoMsg = errors.New("")

// Execute builds a RunContext using ctx and default
// settings, and invokes ExecuteWith.
func Execute(ctx context.Context, stdin *os.File, stdout, stderr io.Writer, args []string) error {
	rc, err := newDefaultRunContext(ctx, stdin, stdout, stderr, args)
	if err != nil {
		printError(rc, err)
		return err
	}

	defer rc.Close() // ok to call rc.Close on nil rc

	ctx = lg.NewContext(ctx, rc.Log)
	return ExecuteWith(ctx, rc, args)
}

// ExecuteWith invokes the cobra CLI framework, ultimately
// resulting in a command being executed. The caller must
// invoke rc.Close.
func ExecuteWith(ctx context.Context, rc *RunContext, args []string) error {
	log := lg.FromContext(ctx)
	log.Debug("EXECUTE", "args", strings.Join(args, " "))
	log.Debug("Build info", "build", buildinfo.Info())
	log.Debug("Config",
		lga.Version, rc.Config.Version,
		lga.Path, rc.ConfigStore.Location(),
	)

	ctx = WithRunContext(ctx, rc)

	rootCmd := newCommandTree(rc)
	var err error

	// The following is a workaround for the fact that cobra doesn't
	// currently (as of 2017, so yeah, "currently") support executing
	// the root command with arbitrary args. That is to say, if you execute:
	//
	//   $ sq @sakila_sl3.actor
	//
	// then cobra will look for a command named "@sakila_sl3.actor",
	// and when it doesn't find such a command, it returns
	// an "unknown command" error.
	//
	// NOTE: This entire mechanism is ancient. Perhaps cobra
	//  now handles this situation?

	// We need to perform handling for autocomplete
	if len(args) > 0 && args[0] == "__complete" {
		if hasMatchingChildCommand(rootCmd, args[1]) {
			// If there is a matching child command, we let rootCmd
			// handle it, as per normal.
			rootCmd.SetArgs(args)
		} else {
			// There's no command matching the first argument to __complete.
			// Therefore, we assume that we want to perform completion
			// for the "slq" command (which is the pseudo-root command).
			effectiveArgs := append([]string{"__complete", "slq"}, args[1:]...)
			rootCmd.SetArgs(effectiveArgs)
		}
	} else {
		var cmd *cobra.Command
		cmd, _, err = rootCmd.Find(args)
		if err != nil {
			// This err will be the "unknown command" error.
			// cobra still returns cmd though. It should be
			// the root cmd.
			if cmd == nil || cmd.Name() != rootCmd.Name() {
				// Not sure if this can happen anymore? Can prob delete?
				panic(fmt.Sprintf("bad cobra cmd state: %v", cmd))
			}

			// If we have args [sq, arg1, arg2] then we redirect
			// to the "slq" command by modifying args to
			// look like: [query, arg1, arg2] -- noting that SetArgs
			// doesn't want the first args element.
			effectiveArgs := append([]string{"slq"}, args...)
			if effectiveArgs, err = preprocessFlagArgVars(effectiveArgs); err != nil {
				return err
			}
			rootCmd.SetArgs(effectiveArgs)
		} else {
			if cmd.Name() == rootCmd.Name() {
				// Not sure why we have two paths to this, but it appears
				// that we've found the root cmd again, so again
				// we redirect to "slq" cmd.
				effectiveArgs := append([]string{"slq"}, args...)
				if effectiveArgs, err = preprocessFlagArgVars(effectiveArgs); err != nil {
					return err
				}
				rootCmd.SetArgs(effectiveArgs)
			} else {
				// It's just a normal command like "sq ls" or such.

				// Explicitly set the args on rootCmd as this makes
				// cobra happy when this func is executed via tests.
				// Haven't explored the reason why.
				rootCmd.SetArgs(args)
			}
		}
	}

	// Execute rootCmd; cobra will find the appropriate
	// sub-command, and ultimately execute that command.
	err = rootCmd.ExecuteContext(ctx)
	if err != nil {
		printError(rc, err)
	}

	return err
}

// cobraMu exists because cobra relies upon package-level
// constructs. This does not sit well with parallel tests.
var cobraMu sync.Mutex

// newCommandTree builds sq's command tree, returning
// the root cobra command.
func newCommandTree(rc *RunContext) (rootCmd *cobra.Command) {
	cobraMu.Lock()
	defer cobraMu.Unlock()

	rootCmd = newRootCmd()
	rootCmd.DisableAutoGenTag = true
	rootCmd.SetOut(rc.Out)
	rootCmd.SetErr(rc.ErrOut)
	rootCmd.Flags().SortFlags = false

	// The --help flag must be explicitly added to rootCmd,
	// or else cobra tries to do its own (unwanted) thing.
	// The behavior of cobra in this regard seems to have
	// changed? This particular incantation currently does the trick.
	rootCmd.Flags().Bool(flag.Help, false, "Show sq help")

	helpCmd := addCmd(rc, rootCmd, newHelpCmd())
	rootCmd.SetHelpCommand(helpCmd)

	// From the end user's perspective, slqCmd is *effectively* the
	// root cmd. We need to perform some trickery to make it output help
	// such that "sq help" and "sq --help" output the same thing.
	slqCmd := newSLQCmd()
	slqCmd.SetHelpFunc(func(command *cobra.Command, i []string) {
		panicOn(rootCmd.Help())
	})

	addCmd(rc, rootCmd, slqCmd)

	addCmd(rc, rootCmd, newSrcAddCmd())
	addCmd(rc, rootCmd, newSrcCommand())
	addCmd(rc, rootCmd, newGroupCommand())
	addCmd(rc, rootCmd, newListCmd())
	addCmd(rc, rootCmd, newMoveCmd())
	addCmd(rc, rootCmd, newRemoveCmd())

	addCmd(rc, rootCmd, newInspectCmd())
	addCmd(rc, rootCmd, newPingCmd())
	addCmd(rc, rootCmd, newSQLCmd())
	addCmd(rc, rootCmd, newScratchCmd())

	tblCmd := addCmd(rc, rootCmd, newTblCmd())
	addCmd(rc, tblCmd, newTblCopyCmd())
	addCmd(rc, tblCmd, newTblTruncateCmd())
	addCmd(rc, tblCmd, newTblDropCmd())

	driverCmd := addCmd(rc, rootCmd, newDriverCmd())
	addCmd(rc, driverCmd, newDriverListCmd())

	configCmd := addCmd(rc, rootCmd, newConfigCmd())
	addCmd(rc, configCmd, newConfigGetCmd())
	addCmd(rc, configCmd, newConfigLocationCmd())
	addCmd(rc, configCmd, newConfigEditCmd())

	addCmd(rc, rootCmd, newCompletionCmd())
	addCmd(rc, rootCmd, newVersionCmd())
	addCmd(rc, rootCmd, newManCmd())

	return rootCmd
}

// hasMatchingChildCommand returns true if s is a full or prefix
// match for any of cmd's children. For example, if cmd has
// children [inspect, ls, rm], then "insp" or "ls" would return true.
func hasMatchingChildCommand(cmd *cobra.Command, s string) bool {
	for _, child := range cmd.Commands() {
		if strings.HasPrefix(child.Name(), s) {
			return true
		}
	}
	return false
}

// addCmd adds the command returned by cmdFn to parentCmd.
func addCmd(rc *RunContext, parentCmd, cmd *cobra.Command) *cobra.Command {
	cmd.Flags().SortFlags = false

	if cmd.Name() != "help" {
		// Don't add the --help flag to the help command.
		cmd.Flags().Bool(flag.Help, false, "help for "+cmd.Name())
	}

	cmd.DisableAutoGenTag = true

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		rc.Cmd = cmd
		rc.Args = args
		err := rc.init()
		return err
	}

	runE := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed(flag.Version) {
			// Bit of a hack: flag --version on any command
			// results in execVersion being invoked
			return execVersion(cmd, args)
		}

		return runE(cmd, args)
	}

	// We handle the errors ourselves (rather than let cobra do it)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true

	parentCmd.AddCommand(cmd)

	return cmd
}

// defaultLogging returns a log (and its associated closer) if
// logging has been enabled via envars.
func defaultLogging() (*slog.Logger, slog.Handler, *cleanup.Cleanup, error) {
	truncate, _ := strconv.ParseBool(os.Getenv(config.EnvarLogTruncate))

	logFilePath, ok := os.LookupEnv(config.EnvarLogPath)
	if !ok || logFilePath == "" || strings.TrimSpace(logFilePath) == "" {
		return lg.Discard(), nil, nil, nil
	}

	// Let's try to create the dir holding the logfile... if it already exists,
	// then os.MkdirAll will just no-op
	parent := filepath.Dir(logFilePath)
	err := os.MkdirAll(parent, 0o750)
	if err != nil {
		return lg.Discard(), nil, nil, errz.Wrapf(err, "failed to create parent dir of log file %s", logFilePath)
	}

	fileFlag := os.O_APPEND
	if truncate {
		fileFlag = os.O_TRUNC
	}

	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|fileFlag, 0o600)
	if err != nil {
		return lg.Discard(), nil, nil, errz.Wrapf(err, "unable to open log file: %s", logFilePath)
	}
	clnup := cleanup.New().AddE(logFile.Close)

	replace := func(groups []string, a slog.Attr) slog.Attr {
		// We want source to be "pkg/file.go".
		if a.Key == slog.SourceKey {
			fp := a.Value.String()
			a.Value = slog.StringValue(filepath.Join(filepath.Base(filepath.Dir(fp)), filepath.Base(fp)))
		}
		return a
	}

	h := slog.HandlerOptions{
		AddSource:   true,
		Level:       slog.LevelDebug,
		ReplaceAttr: replace,
	}.NewJSONHandler(logFile)

	return slog.New(h), h, clnup, nil
}

// printError is the centralized function for printing
// and logging errors. This func has a lot of (possibly needless)
// redundancy; ultimately err will print if non-nil (even if
// rc or any of its fields are nil).
func printError(rc *RunContext, err error) {
	log := lg.Discard()
	if rc != nil && rc.Log != nil {
		log = rc.Log
	}

	if err == nil {
		log.Warn("printError called with nil error")
		return
	}

	if errors.Is(err, errNoMsg) {
		// errNoMsg is a sentinel err that sq doesn't want to print
		return
	}

	switch {
	default:
	case errors.Is(err, context.Canceled):
		err = errz.New("canceled")
	case errors.Is(err, context.DeadlineExceeded):
		err = errz.New("timeout")
	}

	var cmd *cobra.Command
	if rc != nil {
		cmd = rc.Cmd

		cmdName := "unknown"
		if cmd != nil {
			cmdName = cmd.Name()
		}

		lg.Error(log, "nil command", err, lga.Cmd, cmdName)

		wrtrs := rc.writers
		if wrtrs != nil && wrtrs.errw != nil {
			// If we have an errorWriter, we print to it
			// and return.
			wrtrs.errw.Error(err)
			return
		}

		// Else we don't have an errorWriter, so we fall through
	}

	// If we get this far, something went badly wrong in bootstrap
	// (probably the config is corrupt).
	// At this point, we could just print err to os.Stderr and be done.
	// However, our philosophy is to always provide the ability
	// to output errors in json if possible. So, even though cobra
	// may not have initialized and our own config may be borked, we
	// will still try to determine if the user wants the error
	// in json, specified via flags (by directly using the pflag
	// package) or via sq config's default output format.

	opts := options.Options{}
	if rc != nil && rc.Config != nil && rc.Config.Options != nil {
		opts = rc.Config.Options
	} else {
		opts, _ = options.DefaultRegistry.Process(opts)
	}

	// getPrinting works even if cmd is nil
	pr, _, errOut := getPrinting(cmd, opts, os.Stdout, os.Stderr)

	if bootstrapIsFormatJSON(rc) {
		// The user wants JSON, either via defaults or flags.
		jw := jsonw.NewErrorWriter(log, errOut, pr)
		jw.Error(err)
		return
	}

	// The user didn't want JSON, so we just print to stderr.
	if isColorTerminal(os.Stderr) {
		pr.Error.Fprintln(os.Stderr, "sq: "+err.Error())
	} else {
		fmt.Fprintln(os.Stderr, "sq: "+err.Error())
	}
}

// cmdFlagChanged returns true if cmd is non-nil and
// has the named flag and that flag been changed.
func cmdFlagChanged(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}

	f := cmd.Flag(name)
	if f == nil {
		return false
	}

	return f.Changed
}

// cmdFlagTrue returns true if flag name has been changed
// and the flag value is true.
func cmdFlagTrue(cmd *cobra.Command, name string) bool {
	if !cmdFlagChanged(cmd, name) {
		return false
	}

	b, err := cmd.Flags().GetBool(name)
	if err != nil {
		panic(err) // Should never happen
	}

	return b
}

// bootstrapIsFormatJSON is a last-gasp attempt to check if the user
// supplied --json=true on the command line, to determine if a
// bootstrap error (hopefully rare) should be output in JSON.
func bootstrapIsFormatJSON(rc *RunContext) bool {
	// If no RunContext, assume false
	if rc == nil {
		return false
	}

	defaultFormat := format.Table
	if rc.Config != nil {
		defaultFormat = OptOutputFormat.Get(rc.Config.Options)
	}

	// If args were provided, create a new flag set and check
	// for the --json flag.
	if len(rc.Args) > 0 {
		flagSet := pflag.NewFlagSet("bootstrap", pflag.ContinueOnError)

		jsonFlag := flagSet.BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
		err := flagSet.Parse(rc.Args)
		if err != nil {
			return false
		}

		// No --json flag, return true if the config file default is JSON
		if jsonFlag == nil {
			return defaultFormat == format.JSON
		}

		return *jsonFlag
	}

	// No args, return true if the config file default is JSON
	return defaultFormat == format.JSON
}

func panicOn(err error) {
	if err != nil {
		panic(err)
	}
}
