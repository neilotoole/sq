package cli

import (
	"bufio"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/run"
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
	lock, err := ru.Files.CacheLockFor(src)
	if err != nil {
		return err
	}
	fmt.Fprintf(ru.Out, "Locking cache for source %s with timeout %s for %q [%d]\n\n  %s\n\n",
		src.Handle, timeout, os.Args[0], os.Getpid(), lock)

	err = lock.Lock(ctx, timeout)
	if err != nil {
		return err
	}

	fmt.Fprintf(ru.Out, "Cache lock acquired for %s\n", src.Handle)
	fmt.Fprintln(ru.Out, "Press ENTER to release lock and exit.")

	done := make(chan struct{})
	go func() {
		// Wait for ENTER on stdin
		buf := bufio.NewReader(ru.Stdin)
		fmt.Fprint(ru.Out, " > ")
		_, _ = buf.ReadBytes('\n')
		close(done)
	}()

	select {
	case <-done:
		fmt.Fprintln(ru.Out, "ENTER received, releasing lock")
	case <-ctx.Done():
		fmt.Fprintln(ru.Out, "\nContext done, releasing lock")
	}

	fmt.Fprintf(ru.Out, "Releasing cache lock for %s\n", src.Handle)
	if err = lock.Unlock(); err != nil {
		return err
	}

	fmt.Fprintf(ru.Out, "Cache lock released for %s\n", src.Handle)
	return nil
}

func newXDevTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "dev-test",
		Short:   "Execute some dev test code",
		Hidden:  true,
		RunE:    execXDevTestCmd,
		Example: `	$ sq x dev-test`,
	}

	return cmd
}

func execXDevTestCmd(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)
	_ = ru
	return nil
}
