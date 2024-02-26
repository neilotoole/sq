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
	"slices"
	"sync"
	"time"

	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

/*

IMPLEMENTATION NOTE
-------------------

This progress pkg is a fairly hefty wrapper around the vbauerster/mpb pkg, which
does the actual rendering of the progress bars. The development of this pkg has
been a bit of an adventure, with a lot of trial-and-error pain. It's almost
certainly that it could be rebuilt better, but I also never want to go near it
again.

Why not just use the mpb package directly? There are several reasons:

1. At the time of creating this package, the mpb package didn't correctly
   honor the render delay. See: https://github.com/vbauerster/mpb/issues/136
   That bug has since been fixed, but...
2. Delayed initialization of bars is useful for our purposes. In particular, we
   can set the render delay on a per-bar basis, which was not possible with the
   mpb pkg (its render delay is per Progress, not per Bar).
3. The mpb pkg didn't appear to have a mechanism to "hide" a bar temporarily
   while maintaining its counter state. We want to be able to hide a bar, allow
   some main program output to be rendered, then if there's another delay, show
   the hidden bar again. That's not possible if using mpb directly.
4. This pkg has the groupBar mechanism, which is a way to aggregate and then
   disaggregate multiple bars. Basically, once we hit N bars, further bars are
	 aggregated into a single "group" bar. This is useful for UX, as we don't want
	 to clutter the terminal with dozens of progress bars. Also, mpb's performance
	 degrades when there are a large number of bars. With groupBar, the main
   program doesn't have to worry about dozens, hundreds, or even thousands of
   bars being created; they'll just all be aggregated. This simplifies the main
   program logic, because we can just create bars with abandon.
5. Having this wrapper around the mpb package allows us greater flexibility,
   e.g. if we ever want to swap out the mpb package for something else.

Due to battle scars from the development process, you'll find lots of redundant
checks and locks in this pkg. They can probably be tidied away with a little
effort. Also, many interactions with mpb are wrapped in panic-recover. These are
probably overkill, but being that progress bars are merely a UX nicety, we don't
want to the main program to crash due to sloppiness in this package.

TLDR: If you find this pkg's code to be paranoid and/or sloppy, you're probably
correct on both counts. PRs are welcome.

*/

const (
	// uxRedrawFreq is how often the progress bars are redrawn by mpb. The default
	// value is 150ms, which gives pretty smooth animation. Note that this value
	// is different from stateRefreshFreq, which is how often the state of the
	// bars is sent to the mpb widgets.
	uxRedrawFreq = 150 * time.Millisecond

	// stateRefreshFreq is how often the state of the bars is updated and sent to
	// the concrete mpb widgets. Note that every state update puts load on mpb,
	// and it starts to slow down, so refresh shouldn't happen too often.
	stateRefreshFreq = 333 * time.Millisecond

	// DefaultMaxBars is the default threshold at which any further bars are
	// combined into a group bar.
	DefaultMaxBars = 5
)

// Bar represents a single progress bar, owned by a [Progress] instance. The
// caller invokes Incr as necessary to increment the bar's progress. When the
// bar is complete, the caller should invoke Bar.Stop.
type Bar interface {
	// Incr increments the progress bar by amount n.
	Incr(n int)

	// Stop stops and removes the bar, preventing further use of the bar.
	Stop()

	// markShown marks the bar to be displayed.
	markShown()

	// markHidden marks the bar to be hidden.
	markHidden()

	// refresh is called by the Progress's refresh loop to refresh the bar's
	// state.
	refresh(t time.Time)
}

// New returns a new Progress instance, which is a container for progress bars.
// The returned Progress instance is safe for concurrent use, and all of its
// exported methods can be safely invoked on a nil Progress. The caller is
// responsible for calling Progress.Stop on the returned Progress.
//
// Arg delay specifies a duration to wait before rendering the progress bar.
// The Progress is lazily initialized, and thus the delay clock doesn't start
// ticking until the first call to one of the Progress.NewXBar functions.
//
// Arg maxBars specifies the threshold at which any further bars are combined
// into a group bar. This is useful for UX, to avoid flooding the terminal with
// progress bars, and for performance, as the progress widgets don't scale well.
// If maxBars is <= 0, a nil Progress is returned, which won't render any UX.
func New(ctx context.Context, out io.Writer, maxBars int, delay time.Duration, colors *Colors) *Progress {
	if maxBars <= 0 {
		return nil
	}

	log := lg.FromContext(ctx)

	if colors == nil {
		colors = DefaultColors()
	}

	p := &Progress{
		mu:                  &sync.Mutex{},
		colors:              colors,
		align:               newAlignment(),
		allBars:             make([]*virtualBar, 0),
		activeVisibleBars:   make([]*virtualBar, 0),
		activeInvisibleBars: make([]*virtualBar, 0),
		renderDelay:         delay,
		stoppingCh:          make(chan struct{}),
		destroyOnce:         &sync.Once{},
		groupThreshold:      maxBars - 1,
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
	p.groupBar = newGroupBar(p)
	p.startStateRefreshLoop()
	return p
}

// Progress represents a container that renders one or more progress bars.
// The caller is responsible for calling Progress.Stop to indicate
// completion.
type Progress struct {
	ctx      context.Context
	cancelFn context.CancelFunc

	// mu guards ALL public methods.
	mu *sync.Mutex

	// align contains values to visually align progress bar widgets.
	align *alignment

	// stoppingCh is closed at the top of Progress.destroy.
	stoppingCh chan struct{}

	// destroyOnce ensures that Progress.destroy happens only once.
	destroyOnce *sync.Once

	// pc is the underlying mbp.Progress container.
	pc *mpb.Progress

	// colors contains the color scheme to use.
	colors *Colors

	// groupBar is used to aggregate multiple bars into a single group bar, once
	// the number of bars exceeds Progress.groupThreshold.
	groupBar *groupBar

	// allBars contains all non-destroyed virtualBar instances.
	allBars []*virtualBar

	// activeVisibleBars is populated on each state refresh loop with the bars
	// that should be shown.
	activeVisibleBars []*virtualBar

	// activeInvisibleBars is populated on each state refresh loop with the bars
	// that should be aggregated into the group bar.
	activeInvisibleBars []*virtualBar

	// renderDelay is the duration to wait before rendering a progress bar.
	// Each newly-created bar gets its own render delay calculated using the time
	// of bar creation plus this value.
	renderDelay time.Duration

	// groupThreshold is the number of bars after which we combine further bars
	// into a group. We do this because otherwise the terminal output could get
	// filled with dozens of progress bars, which is not great UX. Also, the mpb
	// pkg doesn't seem to handle a large number of bars very well; performance
	// degrades quickly.
	groupThreshold int

	// hidden indicates that the Progress's bars should not be shown.
	hidden bool
}

// Stop waits for all bars to complete and finally shuts down the
// progress container. After this method has been called, there is
// no way to reuse the Progress instance.
func (p *Progress) Stop() {
	if p == nil {
		return
	}

	p.destroy()
}

// Hide hides the Progress's bars.
func (p *Progress) Hide() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.hidden = true
	for _, bar := range p.allBars {
		bar.hide()
	}
}

// Show marks the Progress's bars as eligible for showing.
func (p *Progress) Show() {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.hidden = false
}

// LogValue reports some stats.
func (p *Progress) LogValue() slog.Value {
	var barCount int
	var barsIncrByCallTotal int64
	p.mu.Lock()
	barCount = len(p.allBars)
	for _, bar := range p.allBars {
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

// destroy is probably needlessly complex, but at the time it was written,
// there was a bug in the mpb package (to do with delayed render and abort),
// and so was created an extra-paranoid workaround. Be careful modifying this
// function, as it's a bit of a minefield.
func (p *Progress) destroy() {
	if p == nil {
		return
	}

	p.destroyOnce.Do(func() {
		close(p.stoppingCh)

		defer func() {
			p.cancelFn()
			<-p.ctx.Done()
		}()

		defer func() { _ = recover() }() // Never propagate any panic here

		p.mu.Lock()
		defer p.mu.Unlock()

		if p.pc == nil {
			return
		}

		allBars := slices.Clone(p.allBars)
		wg := &sync.WaitGroup{}
		wg.Add(len(allBars))
		for i := range allBars {
			go func(i int) {
				defer wg.Done()
				allBars[i].destroy()
			}(i)
		}

		p.groupBar.destroy()
		wg.Wait()

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
	})

	<-p.ctx.Done()
}

// BarOpt is a functional option for Bar creation.
type BarOpt interface {
	apply(*Progress, *barConfig)
}

// barConfig is passed to Progress.createBar. Note that there are four decorator
// fields: these are effectively the "widgets" that are displayed on any given
// bar. If a widget is nil, a nopWidget will be set by createVirtualBar. This is
// because we need the widgets to exist (even if invisible) for visual
// alignment purposes.
type barConfig struct {
	style         mpb.BarFillerBuilder
	counterWidget decor.Decorator
	percentWidget decor.Decorator
	timerWidget   decor.Decorator
	memoryWidget  decor.Decorator
	msgWidget     decor.Decorator
	total         int64
}

// createBar creates a new bar, and adds it to Progress.allBars.
//
// The caller must hold Progress.mu.
func (p *Progress) createBar(cfg *barConfig, opts []BarOpt) Bar {
	if p == nil {
		return nopBar{}
	}

	// FIXME: createBar should probably acquire the lock internally.

	vb := newVirtualBar(p, cfg, opts)
	p.allBars = append(p.allBars, vb)
	return vb
}

// forgetBar removes bar vb from Progress.allBars. It is the caller's
// responsibility to first invoke virtualBar.destroy.
func (p *Progress) forgetBar(vb *virtualBar) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.allBars = langz.Remove(p.allBars, vb)
}

// startStateRefreshLoop starts Progress's refresh goroutine, which periodically
// sends the bar states to the concrete mpb widgets. The goroutine returns when
// Progress.ctx or Progress.stoppingCh are done.
func (p *Progress) startStateRefreshLoop() {
	if p == nil {
		return
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				err := errz.Errorf("%v", r)
				// Shouldn't happen, but just in case.
				lg.FromContext(p.ctx).Error("progress state refresh loop panic", lga.Err, err)
			}
		}()

		defer p.Stop()
		done := p.ctx.Done()

		ticker := time.NewTicker(stateRefreshFreq)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-p.stoppingCh:
				return
			case <-ticker.C:
			}

			t := time.Now()

			p.mu.Lock()
			if p.hidden {
				p.mu.Unlock()
				continue
			}

			allBars := slices.Clone(p.allBars)
			p.activeVisibleBars = make([]*virtualBar, 0, p.groupThreshold)
			p.activeInvisibleBars = make([]*virtualBar, 0)

			for i := range allBars {
				bar := allBars[i]
				if !bar.isRenderable(t) {
					continue
				}

				if len(p.activeVisibleBars) < p.groupThreshold {
					bar.wantShow = true
					p.activeVisibleBars = append(p.activeVisibleBars, bar)
					continue
				}

				bar.wantShow = false
				p.activeInvisibleBars = append(p.activeInvisibleBars, bar)
			}

			for i := range allBars {
				select {
				case <-p.stoppingCh:
					p.mu.Unlock()
					return
				case <-done:
					p.mu.Unlock()
					return
				default:
					p.allBars[i].refresh(t)
				}
			}
			p.groupBar.refresh(t)

			p.mu.Unlock()
		}
	}()
}

var _ Bar = nopBar{}

// nopBar is a no-op Bar. It is returned when the Progress is not enabled, so
// that callers don't have to worry about checking for nil.
type nopBar struct{}

func (nopBar) Incr(int)          {}
func (nopBar) Stop()             {}
func (nopBar) markShown()        {}
func (nopBar) markHidden()       {}
func (nopBar) refresh(time.Time) {}
