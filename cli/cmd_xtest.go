package cli

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"

	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/progress"
)

func newXTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "xtest",
		Short:  "Execute some internal tests",
		Hidden: true,
		RunE:   execXTestMbp,
	}

	cmd.Flags().BoolP(flag.JSON, flag.JSONShort, false, flag.JSONUsage)
	cmd.Flags().BoolP(flag.Compact, flag.CompactShort, false, flag.CompactUsage)
	cmd.Flags().BoolP(flag.YAML, flag.YAMLShort, false, flag.YAMLUsage)

	return cmd
}

func execXTestMbp(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	fmt.Fprintln(ru.Out, "Hello, world!")

	pb := progress.New(ctx, ru.ErrOut, 1*time.Millisecond, progress.DefaultColors())
	ctx = progress.NewContext(ctx, pb)

	if err := doProgressByteCounterRead(ctx); err != nil {
		return err
	}

	return ru.Writers.Version.Version(buildinfo.Get(), buildinfo.Get().Version, hostinfo.Get())
}

func doProgressByteCounterRead(ctx context.Context) error {
	pb := progress.FromContext(ctx)

	bar := pb.NewByteCounter("Ingest data test", -1)
	defer bar.Stop()
	maxSleep := 100 * time.Millisecond

	lr := ioz.LimitRandReader(100000)
	b := make([]byte, 1024)

LOOP:
	for {
		select {
		case <-ctx.Done():
			bar.Stop()
			break LOOP
		default:
		}

		n, err := lr.Read(b)
		if err != nil {
			bar.Stop()
			break
		}

		bar.IncrBy(n)
		time.Sleep(time.Duration(rand.Intn(10)+1) * maxSleep / 10) //nolint:gosec
	}

	pb.Stop()
	return nil
}
