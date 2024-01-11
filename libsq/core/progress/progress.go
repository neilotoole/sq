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

// DebugDelay sleeps for a period of time to facilitate testing the
// progress impl. It should be removed before release.
//
// FIXME: Delete this before release.
func DebugDelay() {
	const delay = time.Millisecond * 0
	if delay > 0 {
		time.Sleep(delay)
	}
}

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
	log := lg.FromContext(ctx)
	log.Debug("New progress widget", "delay", delay)

	if colors == nil {
		colors = DefaultColors()
	}

	p := &Progress{
		mu:        &sync.Mutex{},
		colors:    colors,
		bars:      make([]*Bar, 0),
		delay:     delay,
		stoppedCh: make(chan struct{}),
		stopOnce:  &sync.Once{},
		refreshCh: make(chan any, 100),
	}

	// Note that p.ctx is not the same as the arg ctx. This is a bit of a hack
	// to ensure that p.Stop gets called when ctx is cancelled, but before
	// the p.pc learns that its context is cancelled. This was done in an attempt
	// to clean up the progress bars before the main context is cancelled (i.e.
	// to remove bars when the user hits Ctrl-C). Alas, it's not working as
	// hoped in that scenario.
	p.ctx, p.cancelFn = context.WithCancel(lg.NewContext(context.Background(), log))
	go func() {
		<-ctx.Done()
		log.Debug("Stopping via go ctx done")
		p.Stop()
		<-p.stoppedCh
		<-p.ctx.Done()
	}()

	p.pcInitFn = func() {
		opts := []mpb.ContainerOption{
			mpb.WithOutput(out),
			mpb.WithWidth(boxWidth),
			// FIXME: switch back to auto refresh?
			// mpb.WithRefreshRate(refreshRate),
			mpb.WithManualRefresh(p.refreshCh),
			// mpb.WithAutoRefresh(), // Needed for color in Windows, apparently
		}

		p.pc = mpb.NewWithContext(ctx, opts...)
		p.pcInitFn = nil
		go func() {
			for {
				select {
				case <-p.stoppedCh:
					return
				case <-p.ctx.Done():
					return
				default:
					p.refreshCh <- time.Now()
					time.Sleep(refreshRate)
				}
			}
		}()
	}

	p.pcInitFn()
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

	// stoppedCh is closed when the progress widget is stopped.
	// This somewhat duplicates <-p.ctx.Done()... maybe it can be removed?
	stoppedCh chan struct{}
	stopOnce  *sync.Once

	refreshCh chan any

	ctx      context.Context
	cancelFn context.CancelFunc

	// pc is the underlying progress container. It is lazily initialized
	// by pcInitFn. Any method that accesses pc must be certain that
	// pcInitFn has been called.
	pc *mpb.Progress

	// pcInitFn is the func that lazily initializes pc.
	pcInitFn func()

	// delay is the duration to wait before rendering a progress bar.
	// This value is used for each bar created by this Progress.
	delay time.Duration

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
	<-p.stoppedCh
	p.mu.Unlock()
}

// doStop is probably needlessly complex, but at the time it was written,
// there was a bug in the mpb package (to do with delayed render and abort),
// and so was created an extra-paranoid workaround.
func (p *Progress) doStop() {
	p.stopOnce.Do(func() {
		p.pcInitFn = nil
		lg.FromContext(p.ctx).Debug("Stopping progress widget")
		defer lg.FromContext(p.ctx).Debug("Stopped progress widget")
		if p.pc == nil {
			close(p.stoppedCh)
			// close(p.refreshCh)
			p.cancelFn()
			return
		}

		if len(p.bars) == 0 {
			close(p.stoppedCh)
			// close(p.refreshCh)
			p.cancelFn()
			return
		}

		for _, b := range p.bars {
			// We abort each of the bars here, before we call b.doStop() below.
			// In theory, this gives the bar abortion process a head start before
			// b.bar.Wait() is invoked by b.doStop(). This may be completely
			// unnecessary, but it doesn't seem to hurt.
			if b.bar != nil {
				b.bar.SetTotal(-1, true)
				b.bar.Abort(true)
			}
		}

		for _, b := range p.bars {
			b.doStop()
			<-b.barStoppedCh // Wait for bar to stop
		}

		p.refreshCh <- time.Now()
		close(p.stoppedCh)
		// close(p.refreshCh)
		p.pc.Wait()
		// Important: we must call cancelFn after pc.Wait() or the bars
		// may not be removed from the terminal.
		p.cancelFn()
	})

	<-p.stoppedCh
	<-p.ctx.Done()
}

// newBar returns a new Bar. This function must only be called from
// inside the mutex.
func (p *Progress) newBar(msg string, total int64,
	style mpb.BarFillerBuilder, decorators ...decor.Decorator,
) *Bar {
	if p == nil {
		return nil
	}

	select {
	case <-p.stoppedCh:
		return nil
	case <-p.ctx.Done():
		return nil
	default:
	}

	lg.FromContext(p.ctx).Debug("New bar", "msg", msg, "total", total)

	if p.pc == nil {
		p.pcInitFn()
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
		p:            p,
		incrStash:    &atomic.Int64{},
		barInitOnce:  &sync.Once{},
		barStopOnce:  &sync.Once{},
		barStoppedCh: make(chan struct{}),
	}
	b.barInitFn = func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		select {
		case <-p.ctx.Done():
		case <-p.stoppedCh:
			return
		case <-b.barStoppedCh:
			return
		default:
		}

		// REVISIT: It shouldn't be the case that it's possible that the
		// progress has already been stopped. If it is stopped, the call
		// below will panic. Maybe consider wrapping the call in a recover?
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
		b.incrStash = nil
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
	// bar is nil until barInitOnce.Do(barInitFn) is called
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

	barInitOnce *sync.Once
	barInitFn   func()

	barStopOnce  *sync.Once
	barStoppedCh chan struct{}

	delayCh <-chan struct{}

	// incrStash holds the increment count until the
	// bar is fully initialized.
	incrStash *atomic.Int64
}

// IncrBy increments progress by amount of n. It is safe to
// call IncrBy on a nil Bar.
func (b *Bar) IncrBy(n int) {
	if b == nil {
		return
	}

	b.p.mu.Lock()
	defer b.p.mu.Unlock()

	select {
	case <-b.p.stoppedCh:
		return
	case <-b.barStoppedCh:
		return
	case <-b.p.ctx.Done():
		return
	case <-b.delayCh:
		b.barInitOnce.Do(b.barInitFn)
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
	<-b.barStoppedCh
}

func (b *Bar) doStop() {
	if b == nil {
		return
	}

	b.barStopOnce.Do(func() {
		lg.FromContext(b.p.ctx).Debug("Stopping progress bar")
		if b.bar == nil {
			close(b.barStoppedCh)
			return
		}

		// We *probably* only need to call b.bar.Abort() here?
		b.bar.SetTotal(-1, true)
		b.bar.Abort(true)
		b.p.refreshCh <- time.Now()
		b.bar.Wait()
		b.p.refreshCh <- time.Now()

		close(b.barStoppedCh)
		lg.FromContext(b.p.ctx).Debug("Stopped progress bar")
	})
}

// barRenderDelay returns a channel that will be closed after d,
// at which point b will be initialized.
func barRenderDelay(b *Bar, d time.Duration) <-chan struct{} {
	delayCh := make(chan struct{})
	t := time.NewTimer(d)
	go func() {
		defer close(delayCh)
		defer t.Stop()

		<-t.C
		b.barInitOnce.Do(b.barInitFn)
	}()
	return delayCh
}
