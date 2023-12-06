// Package progress contains progress bar widget functionality.
// Use progress.New to create a new progress widget container.
// That widget should be added to a context using progress.NewContext,
// and retrieved via progress.FromContext. Invoke one of the Progress.NewX
// methods to create a new progress.Bar. Invoke Bar.IncrBy to increment
// the bar's progress, and invoke Bar.Stop to stop the bar. Be sure
// to invoke Progress.Stop when the progress widget is no longer needed.
//
// You can use the progress.NewReader and progress.NewWriter functions
// to wrap an io.Reader or io.Writer, respectively, with a progress bar.
// Both functions expect the supplied ctx arg to contain a *progress.Progress.
// Note also that both wrappers are context-aware; that is, they will stop
// the reading/writing process when the context is canceled. Be sure to
// call Close on the wrappers when done.
package progress

import (
	"context"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neilotoole/sq/libsq/core/stringz"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/fatih/color"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/lg"
)

type ctxKey struct{}

// NewContext returns ctx with p added as a value.
func NewContext(ctx context.Context, p *Progress) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the [Progress] added to ctx via NewContext,
// or returns nil. Note that it is safe to invoke the methods
// of a nil [Progress].
func FromContext(ctx context.Context) *Progress {
	if ctx == nil {
		return nil
	}

	val := ctx.Value(ctxKey{})
	if val == nil {
		return nil
	}

	if p, ok := val.(*Progress); ok {
		return p
	}

	return nil
}

const (
	msgLength   = 22
	barWidth    = 28
	boxWidth    = 64
	refreshRate = 150 * time.Millisecond
)

// NOTE: The implementation below is wildly more complicated than it should be.
// This is due to a bug in the mpb package, wherein it doesn't fully
// respect the render delay.
//
//  https://github.com/vbauerster/mpb/issues/136
//
// Until that bug is fixed, we have a messy workaround. The gist of it
// is that both the Progress.pc and Bar.bar are lazily initialized.
// The Progress.pc (progress container) is initialized on the first
// call to one of the Progress.NewX methods. The Bar.bar is initialized
// only after the render delay has expired. The details are ugly.
// Hopefully this can all be simplified once the mpb bug is fixed.

// New returns a new Progress instance, which is a container for progress bars.
// The returned Progress instance is safe for concurrent use, and all of its
// public methods can be safely invoked on a nil Progress. The caller is
// responsible for calling [Progress.Stop] on the returned Progress.
// Arg delay specifies a duration to wait before rendering the progress bar.
// The Progress is lazily initialized, and thus the delay clock doesn't
// start ticking until the first call to one of the Progress.NewX methods.
func New(ctx context.Context, out io.Writer, delay time.Duration, colors *Colors) *Progress {
	lg.FromContext(ctx).Debug("New progress widget", "delay", delay)

	var cancelFn context.CancelFunc
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
		opts := []mpb.ContainerOption{
			mpb.WithOutput(out),
			mpb.WithWidth(boxWidth),
			mpb.WithRefreshRate(refreshRate),
			mpb.WithAutoRefresh(), // Needed for color in Windows, apparently
		}
		if delay > 0 {
			delayCh := renderDelay(p, delay)
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
// The caller is responsible for calling [Progress.Stop] to indicate
// completion.
type Progress struct {
	// mu guards ALL public methods.
	mu *sync.Mutex

	ctx      context.Context
	cancelFn context.CancelFunc

	// pc is the underlying progress container. It is lazily initialized
	// by pcInit. Any method that accesses pc must be certain that
	// pcInit has been called.
	pc *mpb.Progress

	// pcInit is the func that lazily initializes pc.
	pcInit func()

	// delayCh controls the rendering delay: rendering can
	// start as soon as delayCh is closed.
	// TODO: Should delayCh be on Bar instead of Progress?
	delayCh <-chan struct{}

	// stopped is set to true when Stop is called.
	stopped bool

	colors *Colors

	// bars contains all bars that have been created on this Progress.
	bars []*Bar
}

// Stop waits for all bars to complete and finally shuts down the
// container. After this method has been called, there is no way
// to reuse the Progress instance.
func (p *Progress) Stop() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.stopped {
		return
	}

	p.stopped = true
	p.cancelFn()

	if p.pc == nil {
		return
	}

	if len(p.bars) == 0 {
		return
	}

	for _, b := range p.bars {
		if b.bar != nil {
			b.bar.Abort(true)
		}
	}

	for _, b := range p.bars {
		if b.bar != nil {
			b.bar.Wait()
		}
	}

	p.pc.Wait()
}

// initBars lazily initializes all bars in p.bars.
func (p *Progress) initBars() {
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case <-p.ctx.Done():
		return
	default:
	}

	if p.stopped {
		return
	}

	for _, b := range p.bars {
		if !b.stopped {
			b.initBarOnce.Do(b.initBar)
		}
	}
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

// NewUnitTotalCounter returns a new determinate bar whose label
// metric is the plural of the provided unit. The caller is ultimately
// responsible for calling [Bar.Stop] on the returned Bar. However,
// the returned Bar is also added to the Progress's cleanup list, so
// it will be called automatically when the Progress is shut down, but that
// may be later than the actual conclusion of the Bar's work.
//
// This produces output similar to:
//
//	Ingesting sheets   ∙∙∙∙∙●                     4 / 16 sheets
//
// Note that the unit arg is automatically pluralized.
func (p *Progress) NewUnitTotalCounter(msg, unit string, total int64) *Bar {
	if p == nil {
		return nil
	}

	if total <= 0 {
		return p.NewUnitCounter(msg, unit)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	style := barStyle(p.colors.Filler)
	decorator := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current) + " / " + humanize.Comma(statistics.Total)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return s
	})
	decorator = colorize(decorator, p.colors.Size)
	return p.newBar(msg, total, style, decorator)
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

	switch {
	case len(msg) < msgLength:
		msg += strings.Repeat(" ", msgLength-len(msg))
	case len(msg) > msgLength:
		msg = stringz.TrimLenMiddle(msg, msgLength)
	}

	b := &Bar{
		p:           p,
		incrStash:   &atomic.Int64{},
		initBarOnce: &sync.Once{},
	}
	b.initBar = func() {
		if b.stopped || p.stopped {
			return
		}
		b.bar = p.pc.New(total,
			style,
			mpb.BarWidth(barWidth),
			mpb.PrependDecorators(
				colorize(decor.Name(msg, decor.WCSyncWidthR), p.colors.Message),
			),
			mpb.AppendDecorators(decorators...),
			mpb.BarRemoveOnComplete(),
		)
		b.bar.IncrBy(int(b.incrStash.Load()))
		b.incrStash.Store(0)
	}

	p.bars = append(p.bars, b)
	select {
	case <-p.delayCh:
		b.initBarOnce.Do(b.initBar)
	default:
	}
	return b
}

// Bar represents a single progress bar. The caller should invoke
// [Bar.IncrBy] as necessary to increment the bar's progress. When
// the bar is complete, the caller should invoke [Bar.Stop]. All
// methods are safe to call on a nil Bar.
type Bar struct {
	// bar is nil until barInitOnce.Do(initBar) is called
	bar *mpb.Bar
	// p is never nil
	p *Progress

	// There's a bug in the mpb package, wherein it doesn't fully
	// respect the render delay.
	//
	// https://github.com/vbauerster/mpb/issues/136
	//
	// Until that bug is fixed, the Bar is lazily initialized
	// after the render delay expires.

	initBarOnce *sync.Once
	initBar     func()

	// incrStash holds the increment count until the
	// bar is fully initialized.
	incrStash *atomic.Int64

	stopped bool
}

// IncrBy increments progress by amount of n. It is safe to
// call IncrBy on a nil Bar.
func (b *Bar) IncrBy(n int) {
	if b == nil {
		return
	}

	b.p.mu.Lock()
	defer b.p.mu.Unlock()

	if b.stopped || b.p.stopped {
		return
	}

	select {
	case <-b.p.ctx.Done():
		return
	case <-b.p.delayCh:
		b.initBarOnce.Do(b.initBar)
		if b.bar != nil {
			b.bar.IncrBy(n)
		}
		return
	default:
		b.incrStash.Add(int64(n))
	}
}

// Stop stops and removes the bar. It is safe to call Stop on a nil Bar,
// or to call Stop multiple times.
func (b *Bar) Stop() {
	if b == nil {
		return
	}

	b.p.mu.Lock()
	defer b.p.mu.Unlock()

	if b.bar == nil {
		b.stopped = true
		return
	}

	if !b.stopped {
		b.bar.SetTotal(-1, true)
		b.bar.Abort(true)
	}
	b.stopped = true

	b.bar.Wait()
}

// renderDelay returns a channel that will be closed after d,
// at which point p.InitBars will be called.
func renderDelay(p *Progress, d time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	t := time.NewTimer(d)
	go func() {
		defer close(ch)
		defer t.Stop()

		<-t.C
		p.initBars()
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
