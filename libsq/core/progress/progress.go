package progress

import (
	"context"
	"io"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/fatih/color"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/cleanup"
)

type runKey struct{}

// NewContext returns ctx with prog added as a value.
func NewContext(ctx context.Context, prog *Progress) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, runKey{}, prog)
}

// FromContext extracts the Progress added to ctx via NewContext.
func FromContext(ctx context.Context) *Progress {
	if ctx == nil {
		return nil
	}

	val := ctx.Value(runKey{})
	if val == nil {
		return nil
	}

	if p, ok := val.(*Progress); ok {
		return p
	}

	return nil

	// return ctx.Value(runKey{}).(*Progress)
}

func DefaultColors() *Colors {
	return &Colors{
		Message: color.New(color.Faint),
		Spinner: color.New(color.FgGreen, color.Bold),
		Size:    color.New(color.Faint),
		// Percent: color.New(color.FgHiBlue),
		Percent: color.New(color.FgCyan, color.Faint),
		// Percent: color.New(color.FgCyan),
	}
}

type Colors struct {
	Message *color.Color
	Spinner *color.Color
	Size    *color.Color
	Percent *color.Color
}

func (c *Colors) EnableColor(enable bool) {
	if c == nil {
		return
	}

	if enable {
		c.Message.EnableColor()
		c.Spinner.EnableColor()
		c.Size.EnableColor()
		c.Percent.EnableColor()
		return
	}

	c.Message.DisableColor()
	c.Spinner.DisableColor()
	c.Size.DisableColor()
	c.Percent.EnableColor()
}

const (
	barWidth = 28
	boxWidth = 64
)

// New returns a new Progress instance, which is a container for progress bars.
// The returned Progress instance is safe for concurrent use. The caller is
// responsible for calling [Progress.Wait] on the returned Progress.
func New(ctx context.Context, out io.Writer, delay time.Duration, colors *Colors) *Progress {
	p := mpb.NewWithContext(ctx,
		mpb.WithOutput(out),
		mpb.WithWidth(boxWidth),
		mpb.WithRenderDelay(renderDelay(delay)),
	)

	if colors == nil {
		colors = DefaultColors()
	}

	return &Progress{p: p, colors: colors, cleanup: cleanup.New()}
}

// Progress represents a container that renders one or more progress bars.
// The caller is responsible for calling [Progress.Wait] to indicate
// completion.
type Progress struct {
	p       *mpb.Progress
	colors  *Colors
	cleanup *cleanup.Cleanup
}

// Wait waits for all bars to complete and finally shuts down the
// container. After this method has been called, there is no way
// to reuse the Progress instance.
func (p *Progress) Wait() {
	// Invoking cleanup will call Bar.Stop on all the bars.
	_ = p.cleanup.Run()
	p.p.Wait()
}

// NewUnitCounter returns a new indeterminate bar whose label
// metric is the plural of the provided unit. The caller is ultimately
// responsible for calling [Bar.Stop] on the returned Bar. However,
// the returned Bar is also added to the Progress's cleanup list, so
// it will be called automatically when the Progress is shut down, but that
// may be later than the actual conclusion of the spinner's work.
//
//	pbar := p.NewUnitCounter("Ingest records", "record")
//	defer pbar.Stop()
//
//	for i := 0; i < 100; i++ {
//	    pbar.IncrBy(1)
//	    time.Sleep(100 * time.Millisecond)
//	}
//
// This produces output similar to:
//
//	Ingesting records               ∙∙●              87 records
//
// Note that the unit arg is pluralized.
func (p *Progress) NewUnitCounter(msg, unit string) *Bar {
	if p == nil {
		return nil
	}

	decorator := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	})

	decorator = ColorMeta(decorator, p.colors.Size)
	return p.newBar(msg, -1, decorator)
}

// NewByteCounterSpinner returns a new spinner bar whose metric is the count
// of bytes processed. If the size is unknown, set arg size to -1. The caller
// is ultimately responsible for calling [Bar.Stop] on the returned Bar.
// However, the returned Bar is also added to the Progress's cleanup list,
// so it will be called automatically when the Progress is shut down, but that
// may be later than the actual conclusion of the spinner's work.
func (p *Progress) NewByteCounterSpinner(msg string, size int64) *Bar {
	if p == nil {
		return nil
	}

	var counter decor.Decorator
	if size < 0 {
		counter = decor.Current(decor.SizeB1024(0), "% .1f")
	} else {
		counter = decor.Counters(decor.SizeB1024(0), "% .1f / % .1f")
	}
	counter = ColorMeta(counter, p.colors.Size)
	percent := decor.NewPercentage(" %.1f", decor.WCSyncSpace)
	percent = ColorMeta(percent, p.colors.Percent)

	return p.newBar(msg, size, counter, percent)
}

func (p *Progress) newBar(msg string, total int64, decorators ...decor.Decorator) *Bar {
	if p == nil {
		return nil
	}

	if total < 0 {
		total = 0
	}

	style := mpb.SpinnerStyle("∙∙∙", "●∙∙", "∙●∙", "∙∙●", "∙∙∙")
	style = style.Meta(func(s string) string {
		return p.colors.Spinner.Sprint(s)
	})

	bar := p.p.New(total,
		style,
		mpb.BarWidth(barWidth),
		mpb.PrependDecorators(
			ColorMeta(decor.Name(msg, decor.WCSyncWidthR), p.colors.Message),
		),
		mpb.AppendDecorators(decorators...),
		mpb.BarRemoveOnComplete(),
	)

	b := &Bar{bar: bar}
	p.cleanup.Add(b.Stop)
	return b
}

// Bar represents a single progress bar. The caller should invoke
// [Bar.IncrBy] as necessary to increment the bar's progress. When
// the bar is complete, the caller should invoke [Bar.Stop]. All
// methods are safe to call on a nil Bar.
type Bar struct {
	bar *mpb.Bar
}

// IncrBy increments progress by amount of n. It is safe to
// call IncrBy on a nil Bar.
func (b *Bar) IncrBy(n int) {
	if b == nil {
		return
	}
	b.bar.IncrBy(n)
}

// Stop stops and removes the bar. It is safe to call Stop on a nil Bar,
// or to call Stop multiple times.
func (b *Bar) Stop() {
	if b == nil {
		return
	}

	b.bar.SetTotal(-1, true)
	b.bar.Wait()
}

func renderDelay(d time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	time.AfterFunc(d, func() {
		close(ch)
	})
	return ch
}

func ColorMeta(decorator decor.Decorator, c *color.Color) decor.Decorator {
	return decor.Meta(decorator, func(s string) string {
		return c.Sprint(s)
	})
}
