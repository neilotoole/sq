package cli

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func newPingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "ping [@HANDLE [@HANDLE_N]]",
		RunE:              execPing,
		ValidArgsFunction: completeHandle(0),

		Short: "Ping data sources",
		Long: `Ping data sources to check connection health. If no arguments provided, the
active data source is pinged. Provide the handles of one or more sources
to ping those sources, or --all to ping all sources.

The exit code is 1 if ping fails for any of the sources.`,
		Example: `  # ping active data source
  $ sq ping

  # ping all data sources
  $ sq ping --all

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
	cmd.Flags().BoolP(flagJSON, flagJSONShort, false, flagJSONUsage)
	cmd.Flags().Duration(flagPingTimeout, time.Second*10, flagPingTimeoutUsage)
	cmd.Flags().BoolP(flagPingAll, flagPingAllShort, false, flagPingAllUsage)

	return cmd
}

func execPing(cmd *cobra.Command, args []string) error {
	rc := RunContextFrom(cmd.Context())
	cfg := rc.Config
	var srcs []*source.Source

	// args can be:
	// [empty] : ping active source
	// @handle1 @handleN: ping multiple sources

	var pingAll bool
	if cmd.Flags().Changed(flagPingAll) {
		pingAll, _ = cmd.Flags().GetBool(flagPingAll)
	}

	switch {
	case pingAll:
		srcs = cfg.Sources.Items()
	case len(args) == 0:
		src := cfg.Sources.Active()
		if src == nil {
			return errz.New(msgNoActiveSrc)
		}
		srcs = []*source.Source{src}
	default:
		for _, arg := range args {
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
	if cmdFlagChanged(cmd, flagPingTimeout) {
		timeout, _ = cmd.Flags().GetDuration(flagPingTimeout)
	}

	rc.Log.Debugf("Using ping timeout value: %s", timeout)

	return pingSources(cmd.Context(), rc.Log, rc.registry, srcs, rc.writers.pingw, timeout)
}

// pingSources pings each of the sources in srcs, and prints results
// to w. If any error occurs pinging any of srcs, that error is printed
// inline as part of the ping results, and an errNoMsg is returned.
//
// NOTE: This ping code has an ancient lineage, in that it was
// originally laid down before context.Context was a thing. Thus,
// the entire thing could probably be rewritten for simplicity.
func pingSources(ctx context.Context, log lg.Log, dp driver.Provider, srcs []*source.Source, w output.PingWriter,
	timeout time.Duration,
) error {
	if err := w.Open(srcs); err != nil {
		return err
	}
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
		case errors.Is(result.err, context.Canceled):
			// If any one of the goroutines have received context.Canceled,
			// then we'll bubble that up and ignore the remaining goroutines.
			return context.Canceled

		case errors.Is(result.err, context.DeadlineExceeded):
			// If timeout occurred, set the duration to timeout.
			result.duration = timeout
			pingErrExists = true

		case result.err != nil:
			pingErrExists = true
		}

		log.WarnIfError(w.Result(result.src, result.duration, result.err))
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
func pingSource(ctx context.Context, dp driver.Provider, src *source.Source, timeout time.Duration,
	resultCh chan<- pingResult,
) {
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
