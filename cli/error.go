package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"os"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
)

// printError is the centralized function for printing
// and logging errors. This func has a lot of (possibly needless)
// redundancy; ultimately err will print if non-nil (even if
// ru or any of its fields are nil).
func printError(ctx context.Context, ru *run.Run, err error) {
	log := lg.FromContext(ctx)
	log.Warn("printError called", lga.Err, err) // FIXME: delete
	//debug.PrintStack()
	//stack := errz.Stack(err)
	//fmt.Fprintln(ru.Out, "printError stack", "stack", stack)
	if err == nil {
		log.Warn("printError called with nil error")
		return
	}

	if errors.Is(err, errNoMsg) {
		// errNoMsg is a sentinel err that sq doesn't want to print
		return
	}

	switch {
	// Friendlier messages for context errors.
	default:
	case errors.Is(err, context.Canceled):
		err = errz.New("canceled")
	case errors.Is(err, context.DeadlineExceeded):
		err = errz.Wrap(err, "timeout")
	}

	var cmd *cobra.Command
	if ru != nil {
		cmd = ru.Cmd

		cmdName := "unknown"
		if cmd != nil {
			cmdName = cmd.Name()
		}

		if errz.IsErrContext(err) {
			// If it's a context error, e.g. the user cancelled, we'll log it as
			// a warning instead of as an error.
			log.Warn("EXECUTION FAILED", lga.Err, err, lga.Cmd, cmdName)
		} else {
			log.Error("EXECUTION FAILED", lga.Err, err, lga.Cmd, cmdName)
		}

		wrtrs := ru.Writers
		if wrtrs != nil && wrtrs.Error != nil {
			// If we have an errorWriter, we print to it
			// and return.
			wrtrs.Error.Error(err)
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
	if ru != nil && ru.Config != nil && ru.Config.Options != nil {
		opts = ru.Config.Options
	} else if ru != nil && ru.OptionsRegistry != nil {
		opts, _ = ru.OptionsRegistry.Process(opts)
	}

	// getPrinting requires a cleanup.Cleanup, so we get or create one.
	var clnup *cleanup.Cleanup
	if ru != nil && ru.Cleanup != nil {
		clnup = ru.Cleanup
	} else {
		clnup = cleanup.New()
	}
	// getPrinting works even if cmd is nil
	pr, _, errOut := getPrinting(cmd, clnup, opts, os.Stdout, os.Stderr)
	// Execute the cleanup before we print the error.
	if cleanErr := clnup.Run(); cleanErr != nil {
		log.Error("Cleanup failed", lga.Err, cleanErr)
	}

	if bootstrapIsFormatJSON(ru) {
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

// bootstrapIsFormatJSON is a last-gasp attempt to check if the user
// supplied --json=true on the command line, to determine if a
// bootstrap error (hopefully rare) should be output in JSON.
func bootstrapIsFormatJSON(ru *run.Run) bool {
	// If no Run, assume false
	if ru == nil {
		return false
	}

	defaultFormat := format.Text
	if ru.Config != nil {
		defaultFormat = OptFormat.Get(ru.Config.Options)
	}

	// If args were provided, create a new flag set and check
	// for the --json flag.
	if len(ru.Args) > 0 {
		flagSet := pflag.NewFlagSet("bootstrap", pflag.ContinueOnError)

		jsonFlag := flagSet.BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
		err := flagSet.Parse(ru.Args)
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
