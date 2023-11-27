package cli

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/cli/buildinfo"
	"github.com/neilotoole/sq/cli/flag"
	"github.com/neilotoole/sq/cli/hostinfo"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/run"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
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

	if err := doBigRead2(ctx, ru.Writers.Printing, ru.ErrOut); err != nil {
		return err
	}

	//_ = pb

	return ru.Writers.Version.Version(buildinfo.Get(), buildinfo.Get().Version, hostinfo.Get())

}

func execXTestMbpIndeterminate(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	ru := run.FromContext(ctx)

	//bar.Abort(true)
	// wait for our bar to complete and flush
	//p.Wait()
	//bar.Abort(true)
	//p.Shutdown()

	return ru.Writers.Version.Version(buildinfo.Get(), buildinfo.Get().Version, hostinfo.Get())
}

func doBigReadOg(ctx context.Context, errOut io.Writer) error {
	pb := progress.New(ctx, errOut, 1*time.Second)

	//total := 50
	//p := mpb.New(
	//	mpb.WithOutput(ru.Out),
	//	mpb.WithWidth(64),
	//	mpb.WithRenderDelay(after2(1*time.Second)),
	//)
	p := pb.P

	//bar := p.New(
	//	int64(0),
	//	mpb.BarStyle(),
	//	mpb.PrependDecorators(decor.Name("huzzah")),
	//	mpb.BarRemoveOnComplete(),
	//)

	bar := p.AddBar(0,
		mpb.PrependDecorators(decor.Counters(decor.SizeB1024(0), "% .1f / % .1f")),
		//mpb.AppendDecorators(decor.Percentage()),
		mpb.BarRemoveOnComplete(),
	)

	maxSleep := 100 * time.Millisecond

	jr := &junkReader{limit: 1000000}
	b := make([]byte, 1024)

LOOP:
	for {
		select {
		case <-ctx.Done():

			break LOOP
		default:
		}

		n, err := jr.Read(b)
		if err != nil {
			//bar.SetTotal(-1, true)
			if err == io.EOF {
				// triggering complete event now
				bar.SetTotal(-1, true)
				break
			}
			break
		}
		// increment methods won't trigger complete event because bar was constructed with total = 0
		bar.IncrBy(n)
		// following call is not required, it's called to show some progress instead of an empty bar
		bar.SetTotal(bar.Current()+2048, false)
		time.Sleep(time.Duration(rand.Intn(10)+1) * maxSleep / 10)
	}

	p.Wait()
	return nil
}

func doBigRead2(ctx context.Context, pr *output.Printing, errOut io.Writer) error {
	pb := progress.New(ctx, errOut, 1*time.Second)

	//total := 50
	//p := mpb.New(
	//	mpb.WithOutput(ru.Out),
	//	mpb.WithWidth(64),
	//	mpb.WithRenderDelay(after2(1*time.Second)),
	//)
	p := pb.P

	//bar := p.New(
	//	int64(0),
	//	mpb.BarStyle(),
	//	mpb.PrependDecorators(decor.Name("huzzah")),
	//	mpb.BarRemoveOnComplete(),
	//)

	s := mpb.SpinnerStyle("∙∙∙", "●∙∙", "∙●∙", "∙∙●", "∙∙∙")
	spinnerStyle := s.Meta(func(s string) string {
		return pr.Active.Sprint(s)
		//return "\033[31m" + s + "\033[0m" // red
	})

	bar := p.New(0,
		spinnerStyle,
		//mpb.PrependDecorators(),
		//mpb.PrependDecorators(decor.Counters(decor.SizeB1024(0), "% .1f / % .1f")),
		//mpb.PrependDecorators(decor.Counters(decor.SizeB1024(0), "% .1f")),
		mpb.PrependDecorators(
			progress.ColorMeta(decor.Name("Ingesting data..."), pr.Faint),
		),
		//mpb.AppendDecorators(decor.Percentage()),
		mpb.AppendDecorators(
			progress.ColorMeta(decor.Current(decor.SizeB1024(0), "% .1f"), pr.Faint),
		),
		//mpb.AppendDecorators(
		//	// replace ETA decorator with "done" message, OnComplete event
		//	decor.OnComplete(
		//		// ETA decorator with ewma age of 30
		//		decor.EwmaETA(decor.ET_STYLE_GO, 30), "done",
		//	),
		//),
		mpb.BarRemoveOnComplete(),
	)

	maxSleep := 100 * time.Millisecond

	jr := &junkReader{limit: 100000}
	b := make([]byte, 1024)

	//start := time.Now()

LOOP:
	for {
		//bar.EwmaIncrement(time.Since(start))
		select {
		case <-ctx.Done():
			bar.SetTotal(-1, true)
			break LOOP
		default:
		}

		n, err := jr.Read(b)
		if err != nil {
			bar.SetTotal(-1, true)
			if err == io.EOF {
				// triggering complete event now
				//bar.SetTotal(-1, true)
				break
			}
			break
		}
		// increment methods won't trigger complete event because bar was constructed with total = 0
		bar.IncrBy(n)
		// following call is not required, it's called to show some progress instead of an empty bar
		bar.SetTotal(bar.Current()+2048, false)
		time.Sleep(time.Duration(rand.Intn(10)+1) * maxSleep / 10)
	}

	p.Wait()
	return nil
}

func makeStream(limit int) func() (int, error) {
	return func() (int, error) {
		if limit <= 0 {
			return 0, io.EOF
		}
		limit--
		return rand.Intn(1024) + 1, nil
	}
}

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
