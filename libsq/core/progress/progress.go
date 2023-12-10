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

	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

// DebugDelay is a duration that parts of the codebase sleep for to
// facilitate testing the progress impl. It should be removed before
// release.
//
// Deprecated: This is a temporary hack for testing.
const DebugDelay = time.Millisecond * 20

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
		delay:    delay,
	}

	p.pcInit = func() {
		opts := []mpb.ContainerOption{
			mpb.WithOutput(out),
			mpb.WithWidth(boxWidth),
			mpb.WithRefreshRate(refreshRate),
			mpb.WithAutoRefresh(), // Needed for color in Windows, apparently
		}

		p.pc = mpb.NewWithContext(ctx, opts...)
		p.pcInit = nil
	}

	p.pcInit()
	return p
}

// Progress represents a container that renders one or more progress bars.
// The caller is responsible for calling [Progress.Stop] to indicate
// completion.
type Progress struct {
	// The implementation here may seem a bit convoluted. The gist of it is that
	// both the Progress.pc and Bar.bar are lazily initialized. The Progress.pc
	// (progress container) is initialized on the first call to one of the
	// Progress.NewX methods. The Bar.bar is initialized only after the bar's own
	// render delay has expired. The details are ugly.
	//
	// Why not just use the mpb package directly? There are three main reasons:
	//
	// 1. At the time of creating this package, the mpb package didn't correctly
	//    honor the render delay. See: https://github.com/vbauerster/mpb/issues/136
	//    That bug has since been fixed, but...
	// 2. The delayed initialization of the Bar.bar is useful for our purposes.
	//    In particular, we can set the render delay on a per-bar basis, which is
	//    not possible with the mpb package (its render delay is per Progress, not
	//    per Bar).
	// 3. Having this wrapper around the mpb package allows us greater
	//    flexibility, e.g. if we ever want to swap out the mpb package for
	//    something else.

	// mu guards ALL public methods.
	mu *sync.Mutex

	ctx      context.Context
	cancelFn context.CancelFunc

	// pc is the underlying progress container. It is lazily initialized
	// by pcInit. Any method that accesses pc must be certain that
	// pcInit has been called.
	pc *mpb.Progress

	// pcInit is the func that lazily initializes pc.
	// FIXME: Do we even need the lazily initialized pc now?
	pcInit func()

	// delay is the duration to wait before rendering a progress bar.
	// This value is used for each bar created by this Progress.
	delay time.Duration

	// stopped is set to true when Stop is called.
	stopped bool

	colors *Colors

	// bars contains all bars that have been created on this Progress.
	bars []*Bar
}

// Stop waits for all bars to complete and finally shuts down the
// progress container. After this method has been called, there is
// no way to reuse the Progress instance.
func (p *Progress) Stop() {
	if p == nil {
		return
	}

	p.mu.Lock()
	p.doStop()
	p.mu.Unlock()
	lg.FromContext(p.ctx).Debug("Stopped progress widget")
}

// doStop is probably needlessly complex, but at the time it was written,
// there was a bug in the mpb package (to do with delayed render and abort),
// and so was created an extra-paranoid workaround.
func (p *Progress) doStop() {
	if p.stopped {
		return
	}

	p.stopped = true

	if p.pc == nil {
		p.cancelFn()
		return
	}

	if len(p.bars) == 0 {
		// p.pc.Wait() FIXME: Does this need to happen
		p.cancelFn()
		return
	}

	for _, b := range p.bars {
		// We abort each of the bars here, before we call b.doStop() below.
		// In theory, this gives the bar abortion process a head start before
		// b.bar.Wait() is invoked by b.doStop(). This may be completely
		// unnecessary, but it doesn't seem to hurt.
		if b.bar != nil {
			b.bar.Abort(true)
		}
	}

	for _, b := range p.bars {
		b.doStop()
	}

	p.pc.Wait()
	// Important: we must call cancelFn after pc.Wait() or the bars
	// may not be removed from the terminal.
	p.cancelFn()
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

	// We want the bar message to be a consistent width.
	switch {
	case len(msg) < msgLength:
		msg += strings.Repeat(" ", msgLength-len(msg))
	case len(msg) > msgLength:
		msg = stringz.Ellipsify(msg, msgLength)
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

	b.delayCh = barRenderDelay(b, p.delay)
	p.bars = append(p.bars, b)

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

	delayCh <-chan struct{}

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
	case <-b.delayCh:
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

	b.doStop()
	lg.FromContext(b.p.ctx).Debug("Stopped progress bar")
}

func (b *Bar) doStop() {
	if b == nil {
		return
	}

	if b.bar == nil {
		b.stopped = true
		return
	}

	if b.stopped {
		return
	}

	//if !b.stopped {
	//	b.bar.Abort(true)
	//}
	b.bar.SetTotal(-1, true)
	b.bar.Abort(true)
	b.stopped = true

	b.bar.Wait()
}

// barRenderDelay returns a channel that will be closed after d,
// at which point b will be initialized.
func barRenderDelay(b *Bar, d time.Duration) <-chan struct{} {
	ch := make(chan struct{})
	t := time.NewTimer(d)
	go func() {
		defer close(ch)
		defer t.Stop()

		<-t.C
		b.initBarOnce.Do(b.initBar)
	}()
	return ch
}
