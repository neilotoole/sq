package cli

//nolint:unparam,unused,nolintlint
import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/neilotoole/sq/testh/proj"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/files"
)

// newXCmd returns the "x" command, which is the container
// for a set of hidden commands that are useful for development.
// The x commands are not intended for end users.
func newXCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "x",
		Short:  "Run hidden dev/test commands",
		Hidden: true,
	}

	return cmd
}

func newXLockSrcCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "lock-src-cache @src",
		Short:             "Test source cache locking",
		Hidden:            true,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeHandle(1, true),
		RunE:              execXLockSrcCmd,
		Example:           `  $ sq x lock-src-cache @sakila`,
	}

	return cmd
}

func execXLockSrcCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	src, err := ru.Config.Collection.Get(args[0])
	if err != nil {
		return err
	}

	timeout := time.Minute * 20
	ru.Config.Options[files.OptCacheLockTimeout.Key()] = timeout

	unlock, err := ru.Files.CacheLockAcquire(ctx, src)
	if err != nil {
		return err
	}

	fmt.Fprintf(ru.Out, "Cache lock acquired for %s\n", src.Handle)

	select {
	case <-pressEnter(true):
		fmt.Fprintln(ru.Out, "\nENTER received, releasing lock")
	case <-ctx.Done():
		fmt.Fprintln(ru.Out, "\nContext done, releasing lock")
	}

	fmt.Fprintf(ru.Out, "Releasing cache lock for %s\n", src.Handle)
	unlock()

	fmt.Fprintf(ru.Out, "Cache lock released for %s\n", src.Handle)
	return nil
}

func newXProgressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "progress",
		Short:   "Execute progress test code",
		Hidden:  true,
		RunE:    execXProgressManyBars,
		Example: `	$ sq x progress`,
	}

	return cmd
}

const stepSleepy = time.Second * 7 //nolint:unused

func sleepyLog(log *slog.Logger) { //nolint:unused
	log.Warn("Sleeping...", lga.Period, stepSleepy)
	time.Sleep(stepSleepy)
}

func execXProgressHideOnWriter(cmd *cobra.Command, _ []string) error { //nolint:unparam,unused
	ctx := cmd.Context()
	log := lg.FromContext(ctx)
	ru := run.FromContext(ctx)
	_ = log
	_ = ru

	const wantBarCount = 3
	pb := progress.FromContext(ctx)
	var bars []progress.Bar
	// var bar progress.Bar

	for i := 0; i < wantBarCount; i++ {
		bars = append(bars, pb.NewUnitCounter(fmt.Sprintf("counter-%d", i), "item"))
	}

	incrStopCh := make(chan struct{})
	defer close(incrStopCh)
	go func() {
		for ctx.Err() == nil {
			select {
			case <-incrStopCh:
				return
			default:
			}

			for i := range bars {
				bars[i].Incr(1)
			}
			time.Sleep(time.Millisecond * 100)
		}
	}()

	sleepyLog(log)

	log.Warn("writing")
	fmt.Fprintln(ru.Out, "Writing to stdout 0")

	sleepyLog(log)

	fmt.Fprintln(ru.Out, "Writing to stdout 1")
	sleepyLog(log)

	pb.Stop()

	fmt.Fprintln(ru.Out, "exiting")
	return nil
}

//nolint:lll
func execXProgressManyBars(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	log := lg.FromContext(ctx)
	ru := run.FromContext(ctx)
	_ = log
	_ = ru

	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()

	pb := progress.FromContext(ctx)

	bars := make([]progress.Bar, 0)

	bars = append(bars, pb.NewUnitCounter("NewUnitCounter", "item"))
	bars = append(bars, pb.NewUnitCounter("NewUnitCounter.OptTimer", "item", progress.OptTimer))
	bars = append(bars, pb.NewUnitCounter("NewUnitCounter.OptTimer.OptMemUsage", "item", progress.OptTimer, progress.OptMemUsage))

	bars = append(bars, pb.NewWaiter("NewWaiter"))
	bars = append(bars, pb.NewWaiter("NewWaiter.OptMemUsage", progress.OptMemUsage))

	bars = append(bars, pb.NewTimeoutWaiter("NewTimeoutWaiter", time.Now().Add(time.Minute)))
	bars = append(bars, pb.NewTimeoutWaiter("NewTimeoutWaiter.OptTimer", time.Now().Add(time.Minute), progress.OptTimer))
	bars = append(bars, pb.NewTimeoutWaiter("NewTimeoutWaiter.OptTimer.OptMem", time.Now().Add(time.Minute), progress.OptTimer, progress.OptMemUsage))

	bars = append(bars, pb.NewByteCounter("NewByteCounter.Size1000", 1000))
	bars = append(bars, pb.NewByteCounter("NewByteCounter.Size1000.OptTimer", 1000, progress.OptTimer))
	bars = append(bars, pb.NewByteCounter("NewByteCounter.Size1000.OptTimer.OptMem", 1000, progress.OptTimer, progress.OptMemUsage))
	bars = append(bars, pb.NewByteCounter("NewByteCounter.Size1000.OptMem", 1000, progress.OptMemUsage))
	bars = append(bars, pb.NewByteCounter("NewByteCounter.NoSize", -1))
	bars = append(bars, pb.NewByteCounter("NewByteCounter.NoSize.OptTimer", -1, progress.OptTimer))
	bars = append(bars, pb.NewByteCounter("NewByteCounter.NoSize.OptTimer.OptMem", -1, progress.OptTimer, progress.OptMemUsage))
	bars = append(bars, pb.NewByteCounter("NewByteCounter.NoSize.OptMem", -1, progress.OptMemUsage))
	bars = append(bars, pb.NewByteCounter("NewByteCounter.NoSize.OptMem", -1, progress.OptMemUsage))

	fp := proj.Abs("go.mod")
	bars = append(bars, pb.NewFilesizeCounter("NewFilesizeCounter", nil, fp))
	bars = append(bars, pb.NewFilesizeCounter("NewFilesizeCounter.OptTimer", nil, fp, progress.OptTimer))
	bars = append(bars, pb.NewFilesizeCounter("NewFilesizeCounter.OptTimer.OptMem", nil, "fp", progress.OptTimer, progress.OptMemUsage))
	bars = append(bars, pb.NewFilesizeCounter("NewFilesizeCounter.OptMem", nil, fp, progress.OptMemUsage))

	incrStopCh := make(chan struct{})

	go func() {
		for ctx.Err() == nil {
			select {
			case <-incrStopCh:
				return
			default:
			}

			for i := range bars {
				bars[i].Incr(1)
			}
			time.Sleep(time.Millisecond * 100)
		}
	}()

	const stepSleepy = time.Second * 7
	sleepyLog := func() {
		log.Warn("Sleeping...", lga.Period, stepSleepy)
		time.Sleep(stepSleepy)
	}
	_ = sleepyLog

	// <-pressEnter(false)

	log.Warn("DOING THE BIG SLEEP")
	time.Sleep(time.Second * 10)
	log.Warn("BIG SLEEP DONE")

	close(incrStopCh)
	ts := time.Now()
	log.Warn("Stopping")
	pb.Stop()
	log.Warn("Stopped", lga.Elapsed, time.Since(ts))

	fmt.Fprintln(ru.Out, "exiting")
	return nil
}

func execXProgressSingle(cmd *cobra.Command, _ []string) error { //nolint:unparam,unused
	ctx := cmd.Context()
	log := lg.FromContext(ctx)
	ru := run.FromContext(ctx)
	_ = log
	_ = ru

	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)
	renderDelay := OptProgressDelay.Get(options.FromContext(ctx))

	barTimeout := time.Second * 30
	_ = barTimeout
	pb := progress.FromContext(ctx)
	// bar := pb.NewTimeoutWaiter("Doing something...", time.Now().Add(barTimeout))
	bar := pb.NewUnitCounter("Counting stuff...", "thing", progress.OptTimer)
	// defer bar.Stop()

	go func() {
		for ctx.Err() == nil {
			bar.Incr(1)
			time.Sleep(time.Millisecond * 100)
		}
	}()

	const stepSleepy = time.Second * 5
	// bar.Incr(10)

	log.Warn("bar.Show; should be no op")
	progress.ShowBar(bar) // This should be a no-op

	time.Sleep(renderDelay)
	log.Warn("After renderDelay sleep")

	progress.ShowBar(bar)
	log.Warn("Showing bar")
	time.Sleep(stepSleepy)

	log.Warn("Hiding bar")
	progress.HideBar(bar)

	time.Sleep(stepSleepy)

	log.Warn("Showing bar again")
	progress.ShowBar(bar)

	time.Sleep(stepSleepy)

	log.Warn("Stopping bar")
	// bar.Stop()
	defer cancelFn()

	pb.Stop()

	// select {
	// case <-pressEnter():
	// 	bar.Stop()
	// 	pb.Stop()
	// 	fmt.Fprintln(ru.Out, "\nENTER received")
	// case <-ctx.Done():
	// 	bar.Stop()
	// 	pb.Stop()
	// 	fmt.Fprintln(ru.Out, "Context done")
	// case <-time.After(d + time.Second*5):
	// 	bar.Stop()
	// 	log.Warn("timed out, about to print something")
	// 	fmt.Fprintln(ru.Out, "Really timed out")
	// 	log.Warn("done printing")
	// }

	fmt.Fprintln(ru.Out, "exiting")
	return nil
}

func pressEnter(prompt bool) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		buf := bufio.NewReader(os.Stdin)
		if prompt {
			fmt.Fprintf(os.Stdout, "\nPress [ENTER] to continue\n\n  > ")
		}
		_, _ = buf.ReadBytes('\n')
		close(done)
	}()
	return done
}

func newXLockConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "lock-config",
		Short:             "Test config lock",
		Hidden:            true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: completeHandle(1, true),
		RunE: func(cmd *cobra.Command, args []string) error {
			ru := run.FromContext(cmd.Context())
			fmt.Fprintf(ru.Out, "Locking config (pid %d)\n", os.Getpid())
			unlock, err := lockReloadConfig(cmd)
			if err != nil {
				return err
			}

			fmt.Fprintln(ru.Out, "Config locked; ctrl-c to exit")
			<-cmd.Context().Done()
			unlock()
			return nil
		},
		Example: `  $ sq x lock-config`,
	}

	return cmd
}
