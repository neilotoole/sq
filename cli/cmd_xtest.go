package cli

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/spf13/cobra"
	"io"
	"math/rand"
	"time"
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
	//pb := progress.New(ctx, ru.ErrOut, 1*time.Second)

	fmt.Fprintln(ru.Out, "Hello, world!")

	pb := progress.New(ctx, ru.ErrOut, 1*time.Second, progress.DefaultColors())
	ctx = progress.NewContext(ctx, pb)

	if err := doBigRead2(ctx); err != nil {
		return err
	}

	//_ = pb

	return ru.Writers.Version.Version(buildinfo.Get(), buildinfo.Get().Version, hostinfo.Get())

}

func doBigRead2(ctx context.Context) error {
	pb := progress.FromContext(ctx)

	spinner := pb.NewIOSpinner("Ingest data test...")
	defer spinner.Finish()
	maxSleep := 100 * time.Millisecond
	jr := &junkReader{limit: 100000}
	b := make([]byte, 1024)

LOOP:
	for {
		select {
		case <-ctx.Done():
			spinner.Finish()
			break LOOP
		default:
		}

		n, err := jr.Read(b)
		if err != nil {
			spinner.Finish()
			if err == io.EOF {
				break
			}
			break
		}

		spinner.IncrBy(n)
		time.Sleep(time.Duration(rand.Intn(10)+1) * maxSleep / 10)
	}

	//pb.Wait()
	return nil
}

//
//func doBigReadOg(ctx context.Context, errOut io.Writer) error {
//	pb := progress.New(ctx, errOut, 1*time.Second, progress.DefaultColors())
//
//	//total := 50
//	//p := mpb.New(
//	//	mpb.WithOutput(ru.Out),
//	//	mpb.WithWidth(64),
//	//	mpb.WithRenderDelay(after2(1*time.Second)),
//	//)
//	p := pb.P
//
//	//bar := p.New(
//	//	int64(0),
//	//	mpb.BarStyle(),
//	//	mpb.PrependDecorators(decor.Name("huzzah")),
//	//	mpb.BarRemoveOnComplete(),
//	//)
//
//	bar := p.AddBar(0,
//		mpb.PrependDecorators(decor.Counters(decor.SizeB1024(0), "% .1f / % .1f")),
//		//mpb.AppendDecorators(decor.Percentage()),
//		mpb.BarRemoveOnComplete(),
//	)
//
//	maxSleep := 100 * time.Millisecond
//
//	jr := &junkReader{limit: 1000000}
//	b := make([]byte, 1024)
//
//LOOP:
//	for {
//		select {
//		case <-ctx.Done():
//
//			break LOOP
//		default:
//		}
//
//		n, err := jr.Read(b)
//		if err != nil {
//			//bar.SetTotal(-1, true)
//			if err == io.EOF {
//				// triggering complete event now
//				bar.SetTotal(-1, true)
//				break
//			}
//			break
//		}
//		// increment methods won't trigger complete event because bar was constructed with total = 0
//		bar.IncrBy(n)
//		// following call is not required, it's called to show some progress instead of an empty bar
//		bar.SetTotal(bar.Current()+2048, false)
//		time.Sleep(time.Duration(rand.Intn(10)+1) * maxSleep / 10)
//	}
//
//	p.Wait()
//	return nil
//}

type junkReader struct {
	limit int
	count int
}

func (r *junkReader) Read(p []byte) (n int, err error) {
	if r.count >= r.limit {
		return 0, io.EOF
	}

	amount, err := rand.Read(p)
	r.count += amount
	return amount, err

	//return rand.Intn(1024) + 1, nil
}
