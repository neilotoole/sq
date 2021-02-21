package cli

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func newPingCmd() (*cobra.Command, runFunc) {
	argsFn := func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Suggestions are: handles, plus the string "all".
		rc := RunContextFrom(cmd.Context())
		suggestions := append([]string{"all"}, rc.Config.Sources.Handles()...)

		return suggestions, cobra.ShellCompDirectiveNoFileComp
	}

	cmd := &cobra.Command{
		Use: "ping [all|@HANDLE [@HANDLE_N]]",
		//Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: argsFn,

		Short: "Ping data sources",
		Long: `Ping data sources to check connection health. If no arguments provided, the
active data source is pinged. Provide the handles of one or more sources
to ping those sources, or "all" to ping all sources.

The exit code is 1 if ping fails for any of the sources.`,
		Example: `  # ping active data source
  $ sq ping

  # ping all data sources
  $ sq ping all

  # ping @my1 and @pg1
  $ sq ping @my1 @pg1

  # ping @my1 with 2s timeout
  $ sq ping @my1 --timeout=2s

  # output in TSV format
  $ sq ping --tsv @my1`,
	}

	cmd.Flags().BoolP(flagTable, flagTableShort, false, flagTableUsage)
	cmd.Flags().BoolP(flagCSV, flagCSVShort, false, flagCSVUsage)
	cmd.Flags().BoolP(flagTSV, flagTSVShort, false, flagTSVUsage)
	cmd.Flags().Duration(flagTimeout, time.Second*10, flagTimeoutPingUsage)

	return cmd, execPing
}

func execPing(rc *RunContext, cmd *cobra.Command, args []string) error {
	cfg := rc.Config
	var srcs []*source.Source
	var gotAll bool

	// args can be:
	// [empty] : ping active source
	// all     : ping all sources
	// @handle1 @handleN: ping multiple sources
	if len(args) == 0 {
		src := cfg.Sources.Active()
		if src == nil {
			return errz.New(msgNoActiveSrc)
		}
		srcs = []*source.Source{src}
	} else {
		for i, arg := range args {
			if arg == "all" {
				if gotAll || i != 0 {
					// If "all" is an arg, it must the the only one
					return errz.New("arg 'all' must be supplied without other args")
				}

				gotAll = true
				srcs = cfg.Sources.Items()
				continue
			}

			if gotAll {
				// This can happen if arg "all" is mixed in with
				// handle args, e.g. [@handle1 all @handle2]
				return errz.New("arg 'all' must be supplied without other args")
			}

			err := source.VerifyLegalHandle(arg)
			if err != nil {
				return err
			}

			src, err := cfg.Sources.Get(arg)
			if err != nil {
				return err
			}

			srcs = append(srcs, src)
		}
	}

	timeout := cfg.Defaults.PingTimeout
	if cmdFlagChanged(cmd, flagTimeout) {
		timeout, _ = cmd.Flags().GetDuration(flagTimeout)
	}

	rc.Log.Debugf("Using timeout value: %s", timeout)

	return pingSources(rc.Context, rc.Log, rc.registry, srcs, rc.writers.pingw, timeout)
}

// pingSources pings each of the sources in srcs, and prints results
// to w. If any error occurs pinging any of srcs, that error is printed
// inline as part of the ping results, and an errNoMsg is returned.
//
// NOTE: This ping code has an ancient lineage, in that it was written
//  originally laid down before context.Context was a thing. Thus,
//  the entire thing could probably be rewritten for simplicity.
func pingSources(ctx context.Context, log lg.Log, dp driver.Provider, srcs []*source.Source, w output.PingWriter, timeout time.Duration) error {
	w.Open(srcs)
	defer log.WarnIfFuncError(w.Close)

	resultCh := make(chan pingResult, len(srcs))

	// pingErrExists is set to true if there was an error for
	// any of the pings. This later determines if an error
	// is returned from this func.
	var pingErrExists bool

	for _, src := range srcs {
		go pingSource(ctx, dp, src, timeout, resultCh)
	}

	// This func doesn't check for context.Canceled itself; instead
	// it checks if any of the goroutines return that value on
	// resultCh.
	for i := 0; i < len(srcs); i++ {
		result := <-resultCh

		switch {
		case result.err == context.Canceled:
			// If any one of the goroutines have received context.Canceled,
			// then we'll bubble that up and ignore the remaining goroutines.
			return context.Canceled

		case result.err == context.DeadlineExceeded:
			// If timeout occurred, set the duration to timeout.
			result.duration = timeout
			pingErrExists = true

		case result.err != nil:
			pingErrExists = true
		}

		w.Result(result.src, result.duration, result.err)
	}

	// If there's at least one error, we return the
	// sentinel errNoMsg so that sq can os.Exit(1) without printing
	// an additional error message (as the error message will already have
	// been printed by PingWriter).
	if pingErrExists {
		return errNoMsg
	}

	return nil
}

// pingSource pings an individual driver.Source. It always returns a
// result on resultCh, even when ctx is done.
func pingSource(ctx context.Context, dp driver.Provider, src *source.Source, timeout time.Duration, resultCh chan<- pingResult) {
	drvr, err := dp.DriverFor(src.Type)
	if err != nil {
		resultCh <- pingResult{src: src, err: err}
		return
	}

	if timeout > 0 {
		var cancelFn context.CancelFunc
		ctx, cancelFn = context.WithTimeout(ctx, timeout)
		defer cancelFn()
	}

	doneCh := make(chan pingResult)
	start := time.Now()

	go func() {
		err = drvr.Ping(ctx, src)
		doneCh <- pingResult{src: src, duration: time.Since(start), err: err}
	}()

	select {
	case <-ctx.Done():
		resultCh <- pingResult{src: src, err: ctx.Err()}
	case result := <-doneCh:
		resultCh <- result
	}
}

type pingResult struct {
	src      *source.Source
	duration time.Duration
	err      error
}
