// Package cli implements sq's CLI. The spf13/cobra library
// provides the core command processing.
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
// All interaction with cobra should happen inside this package, or
// via the utility cli/cobraz package.
// That is to say, the spf13/cobra package should not be imported
// anywhere outside this package and cli/cobraz.
//
// The entry point to this pkg is the Execute function.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/cobraz"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
)

func init() { //nolint:gochecknoinits
	cobra.EnableCommandSorting = false
}

const (
	msgInvalidArgs       = "invalid args"
	msgNoActiveSrc       = "no active data source"
	msgEmptyQueryString  = "query string is empty"
	msgSrcNoData         = "source has no data"
	msgSrcEmptyTableName = "source has empty table name"
)

// errNoMsg is a sentinel error indicating that a command
// has failed (and thus the program should exit with a non-zero
// code), but no error message should be printed.
// This is useful in the case where any error information may
// already have been printed as part of the command output.
var errNoMsg = errors.New("")

// Execute builds a Run using ctx and default
// settings, and invokes ExecuteWith.
func Execute(ctx context.Context, stdin *os.File, stdout, stderr io.Writer, args []string) error {
	ru, log, err := newRun(ctx, stdin, stdout, stderr, args)
	if err != nil {
		// This may be unnecessary, but we are extra-paranoid about
		// closing ru before exiting the program.
		if closeErr := ru.Close(); closeErr != nil && log != nil {
			log.Error("Failed to close run", lga.Err, closeErr)
		}
		if ru.LogCloser != nil {
			_ = ru.LogCloser()
		}
		PrintError(ctx, ru, err)
		return err
	}

	ctx = lg.NewContext(ctx, log)
	return ExecuteWith(ctx, ru, args)
}

// ExecuteWith invokes the cobra CLI framework, ultimately
// resulting in a command being executed. This function always closes ru.
func ExecuteWith(ctx context.Context, ru *run.Run, args []string) error {
	defer func() {
		if ru != nil && ru.LogCloser != nil {
			_ = ru.LogCloser()
		}
	}()

	ctx = options.NewContext(ctx, options.Merge(options.FromContext(ctx), ru.Config.Options))
	log := lg.FromContext(ctx)
	log.Info("EXECUTE", "args", strings.Join(args, " "))
	log.Info("Build info", "build", buildinfo.Get())
	log.Info("Config", "config_version", ru.Config.Version,
		lga.Path, ru.ConfigStore.Location(),
	)

	ctx = run.NewContext(ctx, ru)

	if freq := OptDebugTrackMemory.Get(options.FromContext(ctx)); freq > 0 {
		// Debug setting to log peak memory usage on exit.
		peakSys, peakAllocs, gcPause := ioz.StartMemStatsTracker(ctx, freq)
		defer func() {
			log.Info("Peak memory stats",
				"sys", datasize.ByteSize(peakSys.Load()).HR(),
				"heap", datasize.ByteSize(peakAllocs.Load()).HR(),
				"gc_pause", time.Duration(gcPause.Load()).String(),
			)
		}()
	}

	rootCmd := newCommandTree(ru)
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

	switch {
	case len(args) > 0 && (args[0] == cobra.ShellCompRequestCmd || args[0] == cobra.ShellCompNoDescRequestCmd):
		// Look for `sq __complete X` or `sq __completeNoDesc X`
		if !OptShellCompletionLog.Get(options.FromContext(ctx)) {
			log.Debug("Discarding shell completion logging",
				lga.Opt, OptShellCompletionLog.Key(), lga.Val, false)
			ctx = lg.NewContext(ctx, lg.Discard())
		}

		if hasMatchingChildCommand(rootCmd, args[1]) {
			// If there is a matching child command, we let rootCmd
			// handle it, as per normal.
			rootCmd.SetArgs(args)
		} else {
			// There's no command matching the first argument
			// to "__complete" / "__completeNoDesc".
			// Therefore, we assume that we want to perform completion
			// for the "slq" command (which is the pseudo-root command).
			effectiveArgs := append([]string{args[0], "slq"}, args[1:]...)
			rootCmd.SetArgs(effectiveArgs)
		}
	case len(args) > 0 && args[0] == cobraz.ShellCompGenScriptsCmd:
		// Look for `sq completion [bash|zsh|X]`, which is cobra's built-in cmd
		// to generate shell completion scripts. This is NOT the same cmd as the
		// cobra.ShellCompRequestCmd ("__complete") which actually returns
		// completion values. The cobraz.ShellCompGenScriptsCmd cmd won't be in the
		// command tree (the call to rootCmd.Find below won't return it), so we need
		// to look specifically for its name in the args, and when found, we
		// let rootCmd take over, which will eventually find that built-in cmd.
		rootCmd.SetArgs(args)
	default:
		var cmd *cobra.Command
		cmd, _, err = rootCmd.Find(args)
		if err != nil {
			// This err will be the "unknown command" error.
			// cobra still returns cmd though. It should be
			// the root cmd.
			if cmd == nil || cmd.Name() != rootCmd.Name() {
				lg.WarnIfCloseError(log, "Problem closing run", ru)
				// Not sure if this can happen anymore? Can prob delete?
				panic(fmt.Sprintf("bad cobra cmd state: %v", cmd))
			}

			// If we have args [sq, arg1, arg2] then we redirect
			// to the "slq" command by modifying args to
			// look like: [query, arg1, arg2] -- noting that SetArgs
			// doesn't want the first args element.
			effectiveArgs := append([]string{"slq"}, args...)
			if effectiveArgs, err = preprocessFlagArgVars(effectiveArgs); err != nil {
				lg.WarnIfCloseError(log, "Problem closing run", ru)
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
					lg.WarnIfCloseError(log, "Problem closing run", ru)
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

	rootCmd.SetContext(ctx)
	// Execute rootCmd; cobra will find the appropriate
	// sub-command, and ultimately execute that command.
	err = rootCmd.ExecuteContext(ctx)
	lg.WarnIfCloseError(log, "Problem closing run", ru)
	if err != nil {
		PrintError(ctx, ru, err)
	}

	return err
}

// cobraMu exists because cobra relies upon package-level
// constructs. This does not sit well with parallel tests.
var cobraMu sync.Mutex

// newCommandTree builds sq's command tree, returning
// the root cobra command.
func newCommandTree(ru *run.Run) (rootCmd *cobra.Command) {
	cobraMu.Lock()
	defer cobraMu.Unlock()

	rootCmd = newRootCmd()
	rootCmd.DisableAutoGenTag = true
	rootCmd.SetOut(ru.Out)
	rootCmd.SetErr(ru.ErrOut)
	rootCmd.Flags().SortFlags = false
	rootCmd.PersistentFlags().SortFlags = false

	helpCmd := addCmd(ru, rootCmd, newHelpCmd())
	rootCmd.SetHelpCommand(helpCmd)

	// From the end user's perspective, slqCmd is *effectively* the
	// root cmd. We need to perform some trickery to make it output help
	// such that "sq help" and "sq --help" output the same thing.
	slqCmd := newSLQCmd()
	slqCmd.SetHelpFunc(func(command *cobra.Command, i []string) {
		panicOn(rootCmd.Help())
	})

	addCmd(ru, rootCmd, slqCmd)

	addCmd(ru, rootCmd, newSrcAddCmd())
	addCmd(ru, rootCmd, newSrcCommand())
	addCmd(ru, rootCmd, newGroupCommand())
	addCmd(ru, rootCmd, newListCmd())
	addCmd(ru, rootCmd, newMoveCmd())
	addCmd(ru, rootCmd, newRemoveCmd())

	addCmd(ru, rootCmd, newInspectCmd())
	addCmd(ru, rootCmd, newPingCmd())
	addCmd(ru, rootCmd, newSQLCmd())
	addCmd(ru, rootCmd, newScratchCmd())

	tblCmd := addCmd(ru, rootCmd, newTblCmd())
	addCmd(ru, tblCmd, newTblCopyCmd())
	addCmd(ru, tblCmd, newTblTruncateCmd())
	addCmd(ru, tblCmd, newTblDropCmd())

	dbCmd := addCmd(ru, rootCmd, newDBCmd())
	dbDumpCmd := addCmd(ru, dbCmd, newDBDumpCmd())
	addCmd(ru, dbDumpCmd, newDBDumpCatalogCmd())
	addCmd(ru, dbDumpCmd, newDBDumpClusterCmd())
	dbRestoreCmd := addCmd(ru, dbCmd, newDBRestoreCmd())
	addCmd(ru, dbRestoreCmd, newDBRestoreCatalogCmd())
	addCmd(ru, dbRestoreCmd, newDBRestoreClusterCmd())

	addCmd(ru, rootCmd, newDiffCmd())

	driverCmd := addCmd(ru, rootCmd, newDriverCmd())
	addCmd(ru, driverCmd, newDriverListCmd())

	configCmd := addCmd(ru, rootCmd, newConfigCmd())
	addCmd(ru, configCmd, newConfigListCmd())
	addCmd(ru, configCmd, newConfigGetCmd())
	addCmd(ru, configCmd, newConfigSetCmd())
	addCmd(ru, configCmd, newConfigLocationCmd())
	addCmd(ru, configCmd, newConfigEditCmd())

	cacheCmd := addCmd(ru, rootCmd, newCacheCmd())
	addCmd(ru, cacheCmd, newCacheLocationCmd())
	addCmd(ru, cacheCmd, newCacheStatCmd())
	addCmd(ru, cacheCmd, newCacheEnableCmd())
	addCmd(ru, cacheCmd, newCacheDisableCmd())
	addCmd(ru, cacheCmd, newCacheClearCmd())
	addCmd(ru, cacheCmd, newCacheTreeCmd())

	// addCmd(ru, rootCmd, newCompletionCmd())
	addCmd(ru, rootCmd, newVersionCmd())
	addCmd(ru, rootCmd, newManCmd())

	xCmd := addCmd(ru, rootCmd, newXCmd())
	addCmd(ru, xCmd, newXLockSrcCmd())
	addCmd(ru, xCmd, newXLockConfigCmd())
	addCmd(ru, xCmd, newXProgressCmd())

	return rootCmd
}

// hasMatchingChildCommand returns true if s is a full or prefix
// match for any of cmd's children. For example, if cmd has
// children [inspect, ls, rm], then "insp" or "ls" would return true.
// Note that there's special handling for cobraz.ShellCompGenScriptsCmd.
func hasMatchingChildCommand(cmd *cobra.Command, s string) bool {
	if strings.HasPrefix(cobraz.ShellCompGenScriptsCmd, s) { //nolint:gocritic // argOrder
		return true
	}

	for _, child := range cmd.Commands() {
		if strings.HasPrefix(child.Name(), s) {
			return true
		}
	}
	return false
}

// addCmd adds the command returned by cmdFn to parentCmd.
func addCmd(ru *run.Run, parentCmd, cmd *cobra.Command) *cobra.Command {
	cmd.DisableFlagsInUseLine = true
	cmd.Flags().SortFlags = false
	cmd.PersistentFlags().SortFlags = false

	if cmd.Name() != "help" {
		// Don't add the --help flag to the help command.
		cmd.Flags().Bool(flag.Help, false, "help for "+cmd.Name())
	}

	cmd.DisableAutoGenTag = true

	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		ru.Cmd = cmd
		ru.Args = args
		return preRun(cmd, ru)
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
