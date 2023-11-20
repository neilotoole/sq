package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/samber/lo"
	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// OptPingCmdTimeout controls timeout for the ping command.
// This timeout applies only to the "sq ping" command: this is a different
// value from driver.OptConnOpenTimeout, which applies to generic ping
// operations.
var OptPingCmdTimeout = options.NewDuration(
	"ping.timeout",
	"",
	0,
	time.Second*10,
	"ping command timeout duration",
	"How long the ping command waits before timeout occurs. Example: 500ms or 2m10s.",
)

func newPingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "ping [@HANDLE|GROUP]*",
		RunE:              execPing,
		ValidArgsFunction: completeHandleOrGroup,

		Short: "Ping data sources",
		Long: `Ping data sources (or groups of sources) to check connection health.
If no arguments provided, the active data source is pinged. Otherwise, ping
the specified sources or groups.

The exit code is 1 if ping fails for any of the sources.`,
		Example: `  # Ping active data source.
  $ sq ping

  # Ping @my1 and @pg1.
  $ sq ping @my1 @pg1

  # Ping sources in the root group (i.e. all sources).
  $ sq ping /

  # Ping sources in the "prod" and "staging" groups.
  $ sq ping prod staging

  # Ping @my1 with 2s timeout.
  $ sq ping @my1 --timeout 2s

  # Output in TSV format.
  $ sq ping --tsv @my1`,
	}

	addTextFlags(cmd)
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.CSV, flag.CSVShort, false, flag.CSVUsage)
	cmd.Flags().Bool(flag.TSV, false, flag.TSVUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)

	cmd.Flags().Duration(flag.PingTimeout, time.Second*10, flag.PingTimeoutUsage)
	return cmd
}

func execPing(cmd *cobra.Command, args []string) error {
	ru := run.FromContext(cmd.Context())
	cfg, coll := ru.Config, ru.Config.Collection
	var srcs []*source.Source

	// args can be:
	// [empty] : ping active source
	// @handle1 @handleN: ping multiple sources
	// @handle1 group1: ping sources, or those in groups.

	args = lo.Uniq(args)
	if len(args) == 0 {
		src := cfg.Collection.Active()
		if src == nil {
			return errz.New(msgNoActiveSrc)
		}
		srcs = []*source.Source{src}
	} else {
		for _, arg := range args {
			switch {
			case source.IsValidHandle(arg):
				src, err := coll.Get(arg)
				if err != nil {
					return err
				}
				srcs = append(srcs, src)
			case source.IsValidGroup(arg):
				groupSrcs, err := coll.SourcesInGroup(arg)
				if err != nil {
					return err
				}

				srcs = append(srcs, groupSrcs...)
			default:
				return errz.Errorf("invalid arg: %s", arg)
			}
		}
	}

	srcs = lo.Uniq(srcs)

	cmdOpts, err := getOptionsFromCmd(cmd)
	if err != nil {
		return err
	}
	timeout := OptPingCmdTimeout.Get(cmdOpts)

	logFrom(cmd).Debug("Using ping timeout", lga.Val, fmt.Sprintf("%v", timeout))

	err = pingSources(cmd.Context(), ru.DriverRegistry, srcs, ru.Writers.Ping, timeout)
	if errors.Is(err, context.Canceled) {
		// It's common to cancel "sq ping". We don't want to print the cancel message.
		return errNoMsg
	}

	return err
}

// pingSources pings each of the sources in srcs, and prints results
// to w. If any error occurs pinging any of srcs, that error is printed
// inline as part of the ping results, and an errNoMsg is returned.
//
// NOTE: This ping code has an ancient lineage, in that it was
// originally laid down before context.Context was a thing. Thus,
// the entire thing could probably be rewritten for simplicity.
func pingSources(ctx context.Context, dp driver.Provider, srcs []*source.Source,
	w output.PingWriter, timeout time.Duration,
) error {
	if err := w.Open(srcs); err != nil {
		return err
	}

	log := lg.FromContext(ctx)
	defer lg.WarnIfFuncError(log, "Close ping writer", w.Close)

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

		err := w.Result(result.src, result.duration, result.err)
		lg.WarnIfError(log, "Print ping result", err)
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
