package cli

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/errz"
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
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	src, err := ru.Config.Collection.Get(args[0])
	if err != nil {
		return err
	}

	timeout := time.Minute * 20
	ru.Config.Options[source.OptCacheLockTimeout.Key()] = timeout

	unlock, err := ru.Files.CacheLockAcquire(ctx, src)
	if err != nil {
		return err
	}

	fmt.Fprintf(ru.Out, "Cache lock acquired for %s\n", src.Handle)

	select {
	case <-pressEnter():
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
		RunE:    execXProgress,
		Example: `	$ sq x progress`,
	}

	return cmd
}

func execXProgress(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	log := lg.FromContext(ctx)
	ru := run.FromContext(ctx)

	d := time.Second * 5
	pb := progress.FromContext(ctx)
	bar := pb.NewTimeoutWaiter("Locking @sakila", time.Now().Add(d))
	defer bar.Stop()

	select {
	case <-pressEnter():
		bar.Stop()
		pb.Stop()
		fmt.Fprintln(ru.Out, "\nENTER received")
	case <-ctx.Done():
		bar.Stop()
		pb.Stop()
		fmt.Fprintln(ru.Out, "Context done")
	case <-time.After(d + time.Second*5):
		bar.Stop()
		log.Warn("timed out, about to print something")
		fmt.Fprintln(ru.Out, "Really timed out")
		log.Warn("done printing")
	}

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
	case len(h.WriteErrors) > 0:
		return h.WriteErrors[0]
	case len(h.CachedFiles) > 0:
		fmt.Fprintf(ru.Out, "Cached: %s\n", h.CachedFiles[0])
		return nil
	case len(h.UncachedFiles) > 0:
		fmt.Fprintf(ru.Out, "Uncached: %d bytes\n", h.UncachedFiles[0].Size())
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

func newXLockConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "lock-config",
		Short:             "Test config lock",
		Hidden:            true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: completeHandle(1),
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
