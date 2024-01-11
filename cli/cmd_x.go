package cli

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/libsq/core/errz"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
	"github.com/neilotoole/sq/libsq/core/ioz/download"
	"github.com/neilotoole/sq/libsq/core/ioz/httpz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/source"
)

// newXCmd returns the root "x" command, which is the container
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
		ValidArgsFunction: completeHandle(1),
		RunE:              execXLockSrcCmd,
		Example:           `  $ sq x lock-src-cache @sakila`,
	}

	return cmd
}

func execXLockSrcCmd(cmd *cobra.Command, args []string) error {
	if unlock, err := lockReloadConfig(cmd); err != nil {
		return err
	} else {
		defer unlock()
	}

	sleep := time.Second * 10
	fmt.Fprintf(os.Stdout, "huzzah, will sleep for %s\n", sleep)
	time.Sleep(sleep)
	return nil

	//ctx := cmd.Context()
	//ru := run.FromContext(ctx)
	//src, err := ru.Config.Collection.Get(args[0])
	//if err != nil {
	//	return err
	//}
	//
	//timeout := time.Minute * 20
	//lock, err := ru.Files.CacheLockFor(src)
	//if err != nil {
	//	return err
	//}
	//fmt.Fprintf(ru.Out, "Locking cache for source %s with timeout %s for %q [%d]\n\n  %s\n\n",
	//	src.Handle, timeout, os.Args[0], os.Getpid(), lock)
	//
	//err = lock.Lock(ctx, timeout)
	//if err != nil {
	//	return err
	//}
	//
	//fmt.Fprintf(ru.Out, "Cache lock acquired for %s\n", src.Handle)
	//
	//select {
	//case <-pressEnter():
	//	fmt.Fprintln(ru.Out, "\nENTER received, releasing lock")
	//case <-ctx.Done():
	//	fmt.Fprintln(ru.Out, "\nContext done, releasing lock")
	//}
	//
	//fmt.Fprintf(ru.Out, "Releasing cache lock for %s\n", src.Handle)
	//if err = lock.Unlock(); err != nil {
	//	return err
	//}
	//
	//fmt.Fprintf(ru.Out, "Cache lock released for %s\n", src.Handle)
	//return nil
}

func newXProgressCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "progress",
		Short:   "Execute progress test code",
		Hidden:  true,
		RunE:    execXProgress,
		Example: `	$ sq x progress`,
	}

	return cmd
}

func execXProgress(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	log := lg.FromContext(ctx)
	ru := run.FromContext(ctx)
	_ = ru

	d := time.Second * 5
	pb := progress.FromContext(ctx)
	bar := pb.NewTimeoutWaiter("Locking @sakila", time.Now().Add(d))
	defer bar.Stop()

	select {
	// case <-pressEnter():
	//	bar.Stop()
	//	pb.Stop()
	//	fmt.Fprintln(ru.Out, "\nENTER received")
	case <-ctx.Done():
		// bar.Stop()
		// pb.Stop()
		fmt.Fprintln(ru.Out, "Context done")
	case <-time.After(d + time.Second*5):
		// bar.Stop()
		log.Warn("timed out, about to print something")
		fmt.Fprintln(ru.Out, "Really timed out")
		log.Warn("done printing")
	}

	// bar.EwmaIncrInt64(rand.Int63n(5)+1, time.Since(start))
	fmt.Fprintln(ru.Out, "exiting")
	return ctx.Err()
}

func newXDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "download URL",
		Short:  "Download a file",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE:   execXDownloadCmd,
		//RunE: func(cmd *cobra.Command, args []string) error {
		//	err1 := errz.New("inner huzzah")
		//	time.Sleep(time.Nanosecond)
		//	err2 := errz.Wrap(err1, "outer huzzah")
		//	time.Sleep(time.Nanosecond)
		//	err3 := errz.Wrap(err2, "outer huzzah")
		//
		//	return err3
		//},
		Example: `  $ sq x download https://sq.io/testdata/actor.csv

  # Download a big-ass file
  $ sq x download https://sqio-public.s3.amazonaws.com/testdata/payment-large.gen.csv
`,
	}
	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	return cmd
}

func execXDownloadCmd(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	log := lg.FromContext(ctx)
	ru := run.FromContext(ctx)

	u, err := url.ParseRequestURI(args[0])
	if err != nil {
		return err
	}

	sum := checksum.Sum([]byte(u.String()))
	fakeSrc := &source.Source{Handle: "@download_" + sum}
	cacheDir, err := ru.Files.CacheDirFor(fakeSrc)
	if err != nil {
		return err
	}

	c := httpz.NewClient(
		httpz.DefaultUserAgent,
		httpz.OptResponseTimeout(time.Second*15),
		// httpz.OptRequestTimeout(time.Second*2),
		httpz.OptRequestDelay(time.Second*5),
	)
	dl, err := download.New(fakeSrc.Handle, c, u.String(), cacheDir)
	if err != nil {
		return err
	}

	h := download.NewSinkHandler(log.With("origin", "handler"))
	dl.Get(ctx, h.Handler)

	switch {
	case len(h.Errors) > 0:
		err1 := errz.Err(h.Errors[0])
		return err1

		//err1 := h.Errors[0]
		//err2 := errz.New("another err")
		//err3 := errz.Combine(err1, err2)
		////lg.FromContext(ctx).Error("OH NO", lga.Err, err3)
		//return err3
		//return nil
	case len(h.WriteErrors) > 0:
		return h.WriteErrors[0]
	case len(h.CachedFiles) > 0:
		fmt.Fprintf(ru.Out, "Cached: %s\n", h.CachedFiles[0])
		return nil
	case len(h.UncachedBufs) > 0:
		fmt.Fprintf(ru.Out, "Uncached: %d bytes\n", h.UncachedBufs[0].Len())
	}

	return nil
}

func pressEnter() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		buf := bufio.NewReader(os.Stdin)
		fmt.Fprintf(os.Stdout, "\nPress [ENTER] to continue\n\n  > ")
		_, _ = buf.ReadBytes('\n')
		close(done)
	}()
	return done
}
