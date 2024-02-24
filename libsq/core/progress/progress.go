// Package progress contains progress bar widget functionality.
// Use progress.New to create a new progress widget container.
// That widget should be added to a context using progress.NewContext,
// and retrieved via progress.FromContext. Invoke one of the Progress.NewX
// methods to create a new progress.Bar. Invoke Bar.Incr to increment
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
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/samber/lo"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/stringz"
)

const (

	// uxRedrawFreq is how often the progress bars are redrawn.
	uxRedrawFreq = 150 * time.Millisecond

	// refreshFreq is how often the state of the bars is refreshed.
	// Experimentation shows 70ms to be appropriate.
	//
	// REVISIT: Confirm 70ms is the appropriate refreshFreq rate.
	refreshFreq = 70 * time.Millisecond

	// groupBarThreshold is the number of bars at which we combine bars into
	// a group. We do this because otherwise the terminal output could get filled
	// with dozens of progress bars, which is not great UX. Also, the mpb package
	// doesn't seem to handle a large number of bars very well; performance goes
	// to hell.
	groupBarThreshold = 5 //nolint:unused
)

// New returns a new Progress instance, which is a container for progress bars.
// The returned Progress instance is safe for concurrent use, and all of its
// public methods can be safely invoked on a nil Progress. The caller is
// responsible for calling Progress.Stop on the returned Progress.
// Arg delay specifies a duration to wait before rendering the progress bar.
// The Progress is lazily initialized, and thus the delay clock doesn't
// start ticking until the first call to one of the Progress.NewX methods.
func New(ctx context.Context, out io.Writer, delay time.Duration, colors *Colors) *Progress {
	log := lg.FromContext(ctx)

	if colors == nil {
		colors = DefaultColors()
	}

	p := &Progress{
		mu:        &sync.Mutex{},
		colors:    colors,
		bars:      make([]*virtualBar, 0),
		delay:     delay,
		stoppedCh: make(chan struct{}),
		stopOnce:  &sync.Once{},
	}

	// Note that p.ctx is not the same as the arg ctx. This is a bit of a hack
	// to ensure that p.Stop gets called when ctx is cancelled, but before
	// the p.pc learns that its context is cancelled. This was done in an attempt
	// to clean up the progress bars before the main context is cancelled (i.e.
	// to remove bars when the user hits Ctrl-C).
	p.ctx, p.cancelFn = context.WithCancel(lg.NewContext(context.Background(), log))

	opts := []mpb.ContainerOption{
		mpb.WithOutput(out),
		mpb.WithWidth(boxWidth),
		mpb.WithAutoRefresh(), // Needed for color in Windows, apparently
		mpb.WithRefreshRate(uxRedrawFreq),
	}

	p.pc = mpb.NewWithContext(ctx, opts...)
	p.startMonitor()
	return p
}

// Progress represents a container that renders one or more progress bars.
// The caller is responsible for calling Progress.Stop to indicate
// completion.
type Progress struct {
	// The implementation here may seem a bit convoluted. When a new bar is
	// created from this Progress, the Bar.bar is initialized only after the
	// bar's own render delay has expired. The details are ugly.
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

	ctx      context.Context
	cancelFn context.CancelFunc

	// mu guards ALL public methods.
	mu *sync.Mutex

	// stoppedCh is closed when the progress widget is stopped.
	// This somewhat duplicates <-p.ctx.Done()... maybe it can be removed?
	stoppedCh chan struct{}
	stopOnce  *sync.Once

	// pc is the underlying progress container. It is lazily initialized
	// by pcInitFn. Any method that accesses pc must be certain that
	// pcInitFn has been called.
	pc *mpb.Progress

	// colors contains the color scheme to use.
	colors *Colors

	groupBar *groupBar //nolint:unused

	// bars contains all bars that have been created on this Progress.
	bars []*virtualBar

	// delay is the duration to wait before rendering a progress bar.
	// Each newly-created bar gets its own render delay.
	delay time.Duration
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

// LogValue reports some stats.
func (p *Progress) LogValue() slog.Value {
	var barCount int
	var barsIncrByCallTotal int64
	p.mu.Lock()
	barCount = len(p.bars)
	for _, bar := range p.bars {
		if bar == nil {
			continue
		}
		barsIncrByCallTotal += bar.incrByCalls.Load()
	}
	p.mu.Unlock()

	return slog.GroupValue(
		slog.Int("bars", barCount),
		slog.Int("incr_by_total", int(barsIncrByCallTotal)),
	)
}

// doStop is probably needlessly complex, but at the time it was written,
// there was a bug in the mpb package (to do with delayed render and abort),
// and so was created an extra-paranoid workaround. It's still not clear
// if all of this works to remove the progress bars before content
// is written to stdout.
func (p *Progress) doStop() {
	p.stopOnce.Do(func() {
		if p.pc == nil {
			p.cancelFn()
			<-p.ctx.Done()
			close(p.stoppedCh)
			return
		}

		if len(p.bars) == 0 {
			p.cancelFn()
			<-p.ctx.Done()
			close(p.stoppedCh)
			return
		}

		for _, b := range p.bars {
			b.destroy()
		}

		// So, now we REALLY want to wait for the progress widget
		// to finish. Alas, the pc.Wait method doesn't seem to
		// always remove the bars from the terminal. So, we do
		// some probably useless extra steps to hopefully trigger
		// the terminal wipe before we return.
		p.pc.Wait()
		// Important: we must call cancelFn after pc.Wait() or the bars
		// may not be removed from the terminal.
		p.cancelFn()
		<-p.ctx.Done()
		// We shouldn't need this extra call to pc.Wait,
		// but it shouldn't hurt?
		p.pc.Wait()
		close(p.stoppedCh)
	})

	<-p.ctx.Done()
}

// Opt is a functional option for Bar creation.
type Opt interface {
	apply(*Progress, *barConfig)
}

// barConfig is passed to Progress.barFromConfig.
type barConfig struct {
	style      mpb.BarFillerBuilder
	msg        string
	decorators []decor.Decorator
	total      int64
}

// barFromConfig returns a bar for cfg. This method must only be called
// from within the Progress mutex. This method may end up calling createVirtualBar,
// or it may return a nopBar, or a groupBar.
func (p *Progress) barFromConfig(cfg *barConfig, opts []Opt) Bar {
	if p == nil {
		return nopBar{}
	}

	return p.createVirtualBar(cfg, opts)
}

// createVirtualBar returns a new virtualBar (or nil). It must only be called
// from inside the Progress mutex. Generally speaking, callers should use
// Progress.barFromConfig instead of calling createVirtualBar directly.
func (p *Progress) createVirtualBar(cfg *barConfig, opts []Opt) *virtualBar {
	if p == nil {
		return nil
	}

	cfg.decorators = lo.WithoutEmpty(cfg.decorators)

	select {
	case <-p.stoppedCh:
		return nil
	case <-p.ctx.Done():
		return nil
	default:
	}

	if cfg.total < 0 {
		cfg.total = 0
	}

	// We want the bar message to be a consistent width.
	switch {
	case len(cfg.msg) < msgLength:
		cfg.msg += strings.Repeat(" ", msgLength-len(cfg.msg))
	case len(cfg.msg) > msgLength:
		cfg.msg = stringz.Ellipsify(cfg.msg, msgLength)
	}

	for _, opt := range opts {
		if opt != nil {
			opt.apply(p, cfg)
		}
	}

	b := &virtualBar{
		p:           p,
		incrByCalls: &atomic.Int64{},
		incrTotal:   &atomic.Int64{},
		destroyOnce: &sync.Once{},
		notBefore:   time.Now().Add(p.delay),
		cfg:         cfg,
	}

	p.bars = append(p.bars, b)
	b.show()
	return b
}

// startMonitor starts Progress's monitor goroutine, which periodically
// refreshes the bars. The goroutine returns when p.ctx or p.stoppedCh are done.
func (p *Progress) startMonitor() {
	if p == nil {
		return
	}

	ctx := p.ctx
	go func() {
		defer p.Stop()

		done := ctx.Done()
		for {
			select {
			case <-done:
				return
			case <-p.stoppedCh:
				return
			default:
			}

			for i := 0; i < len(p.bars); i++ {
				select {
				case <-done:
					return
				case <-p.stoppedCh:
					return
				default:
					if p.bars[i] == nil {
						continue
					}
					p.bars[i].refresh()
				}
			}

			time.Sleep(refreshFreq)
		}
	}()
}
