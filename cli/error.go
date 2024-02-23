package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/neilotoole/sq/cli/output"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output/format"
	"github.com/neilotoole/sq/cli/output/jsonw"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/cleanup"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/termz"
)

// PrintError is the centralized function for printing
// and logging errors. This func has a lot of (possibly needless)
// redundancy; ultimately err will print if non-nil (even if
// ctx, or ru, or any of ru's fields are nil).
func PrintError(ctx context.Context, ru *run.Run, err error) {
	if ctx == nil {
		ctx = context.Background()
	}

	log := lg.FromContext(ctx)

	if err == nil {
		log.Warn("PrintError called with nil error")
		return
	}

	if errors.Is(err, errNoMsg) {
		// errNoMsg is a sentinel err that sq doesn't want to print
		return
	}

	cmdName := "unknown"
	var cmd *cobra.Command
	if ru != nil && ru.Cmd != nil {
		cmd = ru.Cmd
		cmdName = ru.Cmd.Name()
	}

	log.Error("EXECUTION FAILED",
		lga.Err, err, lga.Cmd, cmdName, lga.Stack, errz.Stacks(err))

	humanErr := humanizeError(err)
	if ru != nil {
		if wrtrs := ru.Writers; wrtrs != nil && wrtrs.Error != nil {
			// If we have an errorWriter, we print to it
			// and return.
			wrtrs.Error.Error(err, humanErr)
			return
		}

		// Else we don't have an error writer, so we fall through
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
	// getOutputConfig works even if cmd is nil
	fm := getFormat(cmd, opts)
	var errOut io.Writer
	var pr *output.Printing
	if ru != nil && ru.Stdout != nil && ru.Stderr != nil {
		outCfg := getOutputConfig(cmd, clnup, fm, opts, ru.Stdout, ru.Stderr)
		if outCfg != nil {
			errOut, pr = outCfg.errOut, outCfg.errOutPr
		}
	}
	if errOut == nil {
		errOut = os.Stderr
	}
	if pr == nil {
		pr = output.NewPrinting()
	}

	// Execute the cleanup before we print the error.
	if cleanErr := clnup.Run(); cleanErr != nil {
		log.Error("Cleanup failed", lga.Err, cleanErr)
	}

	if bootstrapIsFormatJSON(ru) {
		// The user wants JSON, either via defaults or flags.
		jw := jsonw.NewErrorWriter(log, errOut, pr)
		jw.Error(err, humanErr)
		return
	}

	// The user didn't want JSON, so we just print to stderr.
	if termz.IsColorTerminal(os.Stderr) {
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

// humanizeError wrangles an error to make it more human-friendly before
// printing to stderr. The returned err may be a different error from the
// one passed in. This should be the final step before printing an error;
// the original error should have already been logged.
func humanizeError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	// Friendlier messages for context errors.
	default:
	case errors.Is(err, context.Canceled):
		err = errz.New("canceled")
	case errors.Is(err, context.DeadlineExceeded):
		errMsg := err.Error()
		deadlineMsg := context.DeadlineExceeded.Error()
		if errMsg == deadlineMsg {
			// For generic context.DeadlineExceeded errors, we
			// just return "timeout".
			err = errz.New("timeout")
		} else {
			// But if the error is a wrapped context.DeadlineExceeded, we
			// trim off the ": context deadline exceeded" suffix.
			return errz.New(strings.TrimSuffix(errMsg, ": "+deadlineMsg))
		}
	}

	return err
}
