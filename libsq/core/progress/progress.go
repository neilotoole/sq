package progress

import (
	"context"
	"io"
	"sync"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/fatih/color"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/lg"
)

type runKey struct{}

// NewContext returns ctx with prog added as a value.
func NewContext(ctx context.Context, prog *Progress) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, runKey{}, prog)
}

// FromContext returns the [Progress] added to ctx via NewContext,
// or returns nil. Note that it is safe to invoke the methods
// of a nil [Progress].
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
}

const (
	barWidth    = 28
	boxWidth    = 64
	refreshRate = 150 * time.Millisecond
)

// New returns a new Progress instance, which is a container for progress bars.
// The returned Progress instance is safe for concurrent use, and all of its
// public methods can be safely invoked on a nil Progress. The caller is
// responsible for calling [Progress.Wait] on the returned Progress.
// Arg delay specifies a duration to wait before rendering the progress bar.
// The Progress is lazily initialized, and thus the delay clock doesn't
// start ticking until the first call to one of the Progress.NewX methods.
func New(ctx context.Context, out io.Writer, delay time.Duration, colors *Colors) *Progress {
	lg.FromContext(ctx).Debug("New progress widget", "delay", delay)

	var cancelFn context.CancelFunc
	ogCtx := ctx
	_ = ogCtx
	ctx, cancelFn = context.WithCancel(ctx)

	if colors == nil {
		colors = DefaultColors()
	}

	p := &Progress{
		ctx:      ctx,
		mu:       &sync.Mutex{},
		colors:   colors,
		cancelFn: cancelFn,
		bars:     make([]*Bar, 0),
	}

	p.pcInit = func() {
		lg.FromContext(ctx).Debug("Initializing progress widget")
		opts := []mpb.ContainerOption{
			mpb.WithOutput(out),
			mpb.WithWidth(boxWidth),
			mpb.WithRefreshRate(refreshRate),
			mpb.WithAutoRefresh(), // Needed for color in Windows, apparently
		}
		if delay > 0 {
			delayCh := renderDelay(ctx, delay)
			opts = append(opts, mpb.WithRenderDelay(delayCh))
			p.delayCh = delayCh
		} else {
			delayCh := make(chan struct{})
			close(delayCh)
			p.delayCh = delayCh
		}

		p.pc = mpb.NewWithContext(ctx, opts...)
		p.pcInit = nil
	}
	return p
}

// Progress represents a container that renders one or more progress bars.
// The caller is responsible for calling [Progress.Wait] to indicate
// completion.
type Progress struct {
	// mu guards ALL public methods.
	mu *sync.Mutex

	ctx context.Context

	// pc is the underlying progress container. It is lazily initialized
	// by pcInit. Any method that accesses pc must be certain that
	// pcInit has been called.
	pc *mpb.Progress

	// pcInit is the func that lazily initializes pc.
	pcInit func()

	// delayCh controls the rendering delay: rendering can
	// start as soon as delayCh is closed.
	delayCh <-chan struct{}

	colors *Colors
	// cleanup *cleanup.Cleanup
	bars []*Bar

	cancelFn context.CancelFunc
}

// Wait waits for all bars to complete and finally shuts down the
// container. After this method has been called, there is no way
// to reuse the Progress instance.
func (p *Progress) Wait() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.pc == nil {
		return
	}

	if len(p.bars) == 0 {
		return
	}

	p.cancelFn()

	for _, bar := range p.bars {
		bar.bar.Abort(true)
	}

	for _, bar := range p.bars {
		bar.bar.Wait()
	}

	p.pc.Wait()
}

// NewUnitCounter returns a new indeterminate bar whose label
// metric is the plural of the provided unit. The caller is ultimately
// responsible for calling [Bar.Stop] on the returned Bar. However,
// the returned Bar is also added to the Progress's cleanup list, so
// it will be called automatically when the Progress is shut down, but that
// may be later than the actual conclusion of the spinner's work.
//
//	bar := p.NewUnitCounter("Ingest records", "rec")
//	defer bar.Stop()
//
//	for i := 0; i < 100; i++ {
//	    bar.IncrBy(1)
//	    time.Sleep(100 * time.Millisecond)
//	}
//
// This produces output similar to:
//
//	Ingesting records               ∙∙●              87 recs
//
// Note that the unit arg is automatically pluralized.
func (p *Progress) NewUnitCounter(msg, unit string) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	decorator := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	})
	decorator = colorize(decorator, p.colors.Size)

	style := spinnerStyle(p.colors.Filler)

	return p.newBar(msg, -1, style, decorator)
}

// NewByteCounter returns a new progress bar whose metric is the count
// of bytes processed. If the size is unknown, set arg size to -1. The caller
// is ultimately responsible for calling [Bar.Stop] on the returned Bar.
// However, the returned Bar is also added to the Progress's cleanup list,
// so it will be called automatically when the Progress is shut down, but that
// may be later than the actual conclusion of the Bar's work.
func (p *Progress) NewByteCounter(msg string, size int64) *Bar {
	if p == nil {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	var style mpb.BarFillerBuilder
	var counter decor.Decorator
	var percent decor.Decorator
	if size < 0 {
		style = spinnerStyle(p.colors.Filler)
		counter = decor.Current(decor.SizeB1024(0), "% .1f")
	} else {
		style = barStyle(p.colors.Filler)
		counter = decor.Counters(decor.SizeB1024(0), "% .1f / % .1f")
		percent = decor.NewPercentage(" %.1f", decor.WCSyncSpace)
		percent = colorize(percent, p.colors.Percent)
	}
	counter = colorize(counter, p.colors.Size)

	return p.newBar(msg, size, style, counter, percent)
}

// newBar returns a new Bar. This function must only be called from
// inside the mutex.
func (p *Progress) newBar(msg string, total int64,
	style mpb.BarFillerBuilder, decorators ...decor.Decorator,
) *Bar {
	if p == nil {
		return nil
	}

	lg.FromContext(p.ctx).Debug("New bar", "msg", msg, "total", total)

	select {
	case <-p.ctx.Done():
		return nil
	default:
	}

	if p.pc == nil {
		p.pcInit()
	}

	if total < 0 {
		total = 0
	}

	bar := p.pc.New(total,
		style,
		mpb.BarWidth(barWidth),
		mpb.PrependDecorators(
			colorize(decor.Name(msg, decor.WCSyncWidthR), p.colors.Message),
		),
		mpb.AppendDecorators(decorators...),
		mpb.BarRemoveOnComplete(),
	)

	b := &Bar{p: p, bar: bar}
	p.bars = append(p.bars, b)
	return b
}

// barStopped is called by a Bar when it is stopped.
// This was supposed to do something, but it's a no-op for now.
func (p *Progress) barStopped(_ *Bar) {
	if p == nil {
		return
	}
}

func spinnerStyle(c *color.Color) mpb.SpinnerStyleComposer {
	// REVISIT: maybe use ascii chars only, in case it's a weird terminal?
	frames := []string{"∙∙∙", "●∙∙", "●∙∙", "∙●∙", "∙●∙", "∙∙●", "∙∙●", "∙∙∙"}
	style := mpb.SpinnerStyle(frames...)
	if c != nil {
		style = style.Meta(func(s string) string {
			return c.Sprint(s)
		})
	}
	return style
}

func barStyle(c *color.Color) mpb.BarStyleComposer {
	clr := func(s string) string {
		return c.Sprint(s)
	}

	frames := []string{"∙", "●", "●", "●", "∙"}

	return mpb.BarStyle().
		Lbound("  ").Rbound("  ").
		Filler("∙").FillerMeta(clr).
		Padding(" ").
		Tip(frames...).TipMeta(clr)
}

// Bar represents a single progress bar. The caller should invoke
// [Bar.IncrBy] as necessary to increment the bar's progress. When
// the bar is complete, the caller should invoke [Bar.Stop]. All
// methods are safe to call on a nil Bar.
type Bar struct {
	p   *Progress
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
	b.bar.Abort(true)
	b.bar.Wait()
	b.p.barStopped(b)
}

// renderDelay returns a channel that will be closed after d,
// or if ctx is done.
func renderDelay(ctx context.Context, d time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	t := time.NewTimer(d)
	go func() {
		defer close(ch)
		defer t.Stop()
		select {
		case <-ctx.Done():
			lg.FromContext(ctx).Debug("Render delay via ctx.Done")
		case <-t.C:
			lg.FromContext(ctx).Debug("Render delay via timer")
		}
	}()
	return ch
}

func colorize(decorator decor.Decorator, c *color.Color) decor.Decorator {
	return decor.Meta(decorator, func(s string) string {
		return c.Sprint(s)
	})
}

// DefaultColors returns the default colors used for the progress bars.
func DefaultColors() *Colors {
	return &Colors{
		Message: color.New(color.Faint),
		Filler:  color.New(color.FgGreen, color.Bold, color.Faint),
		Size:    color.New(color.Faint),
		Percent: color.New(color.FgCyan, color.Faint),
	}
}

// Colors is the set of colors used for the progress bars.
type Colors struct {
	Message *color.Color
	Filler  *color.Color
	Size    *color.Color
	Percent *color.Color
}

// EnableColor enables or disables color for the progress bars.
func (c *Colors) EnableColor(enable bool) {
	if c == nil {
		return
	}

	if enable {
		c.Message.EnableColor()
		c.Filler.EnableColor()
		c.Size.EnableColor()
		c.Percent.EnableColor()
		return
	}

	c.Message.DisableColor()
	c.Filler.DisableColor()
	c.Size.DisableColor()
	c.Percent.DisableColor()
}
