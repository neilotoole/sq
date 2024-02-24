package progress

import (
	"sync"
	"sync/atomic"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// Bar represents a single progress bar. The caller should invoke Incr as
// necessary to increment the bar's progress. When the bar is complete, the
// caller should invoke Bar.Stop.
type Bar interface {
	// Incr increments the progress bar by amount n.
	Incr(n int)

	// Stop stops and removes the bar, preventing further use of the bar.
	Stop()

	// show shows the bar. It may be hidden again using Bar.hide.
	show()

	// hide hides the bar. It may be shown again using Bar.show.
	hide()

	// refresh is called by the Progress's monitor loop to refresh the bar's
	// state.
	refresh()
}

var _ Bar = nopBar{}

// nopBar is a no-op Bar. It is returned when the Progress is not enabled, so
// that callers don't have to worry about checking for nil.
type nopBar struct{}

func (nopBar) Incr(int) {}
func (nopBar) Stop()    {}
func (nopBar) show()    {}
func (nopBar) hide()    {}
func (nopBar) refresh() {}

var _ Bar = (*virtualBar)(nil)

// virtualBar is the main implementation of Bar. It is a virtual bar in the
// sense that it is not a concrete mpb.Bar, but rather an abstraction that
// may create and destroy a concrete mpb.Bar as necessary, as virtualBar.show
// or virtualBar.hide are invoked.
type virtualBar struct {
	// notBefore is a checkpoint before which the virtualBar isn't shown. It's
	// basically a render delay for the virtualBar.
	notBefore time.Time

	// incrLastSentTime is when incrLastSentVal was last sent to bimpl.
	incrLastSentTime time.Time

	// cfg is the bar's configuration. It is preserved so that the bar can
	// be hidden and shown.
	cfg *barConfig

	// bimpl is the concrete mpb.Bar impl. While the virtualBar is hidden, or
	// destroyed, bimpl is nil. While the virtualBar is shown, bimpl is non-nil.
	bimpl *mpb.Bar

	// p is the virtualBar's parent Progress.
	p *Progress

	// incrTotal holds the total value of increment values passed to Incr.
	incrTotal *atomic.Int64

	// incrByCalls is a count of virtualBar.Incr invocations. It's used for
	// logging stats.
	incrByCalls *atomic.Int64

	// destroyOnce is used within virtualBar.destroy.
	destroyOnce *sync.Once

	// incrLastSentVal is the most recent value sent to bimpl.
	incrLastSentVal int64

	// mu guards the virtualBar's fields.
	mu sync.Mutex

	// destroyed is set to true by virtualBar.destroy
	destroyed bool

	// wantShow is true if we want the virtualBar to be shown. However, even if
	// true, the bar may not be shown if notBefore hasn't been reached.
	wantShow bool
}

// Incr increments the progress bar by amount n.
func (b *virtualBar) Incr(n int) {
	if b == nil {
		return
	}

	b.incrTotal.Add(int64(n))
	b.incrByCalls.Add(1)
}

func (b *virtualBar) refresh() {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.destroyed {
		return
	}

	if !b.wantShow {
		if b.bimpl != nil {
			b.stopConcrete()
		}
		return
	}

	if b.bimpl == nil {
		b.doShow()
		// REVISIT: Hmmn, I think we always want to call maybeSendIncr here.
		// return
	}

	b.maybeSendIncr()
}

func (b *virtualBar) maybeSendIncr() {
	if b == nil || b.bimpl == nil || !b.wantShow {
		return
	}

	total := b.incrTotal.Load()
	amount := total - b.incrLastSentVal
	if amount == 0 {
		return
	}

	b.bimpl.IncrBy(int(amount))
	b.incrLastSentVal = total
	b.incrLastSentTime = time.Now()
}

func (b *virtualBar) show() {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.doShow()
}

// doShow must be called inside b's mutex.
func (b *virtualBar) doShow() {
	if b == nil || b.destroyed {
		return
	}

	b.wantShow = true
	if b.bimpl != nil {
		return
	}

	if !time.Now().After(b.notBefore) {
		return
	}

	b.startConcrete()
}

// startConcrete start's the virtualBar's concrete mpb.Bar. It must be called
// inside b's mutex.
func (b *virtualBar) startConcrete() {
	defer func() {
		if r := recover(); r != nil {
			// On a previous version of this codebase, we would occasionally see
			// panics due to a race condition. This recover was added to paper over
			// the panic, but it's not clear if it's still necessary.
			err := errz.Errorf("progress: %v", r)
			lg.FromContext(b.p.ctx).Warn("Caught panic in progress.startConcrete", lga.Err, err)
		}
	}()

	b.bimpl = b.p.pc.New(b.cfg.total,
		b.cfg.style,
		mpb.BarWidth(barWidth),
		mpb.PrependDecorators(
			colorize(decor.Name(b.cfg.msg, decor.WCSyncWidthR), b.p.colors.Message),
		),
		mpb.AppendDecorators(b.cfg.decorators...),
		mpb.BarRemoveOnComplete(),
	)

	// Send the total value to the bar.
	total := b.incrTotal.Load()
	b.incrLastSentVal = total
	b.incrLastSentTime = time.Now()
	b.bimpl.IncrBy(int(total))
}

func (b *virtualBar) hide() {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.wantShow = false
	b.stopConcrete()
}

func (b *virtualBar) stopConcrete() {
	if b == nil {
		return
	}

	b.wantShow = false
	if b.bimpl == nil {
		return
	}

	// We *probably* only need to call b.bar.Abort() here?
	b.bimpl.SetTotal(-1, true)
	b.bimpl.Abort(true)
	b.bimpl.Wait()
	lg.FromContext(b.p.ctx).Warn("Hiding bar", "bar msg", b.cfg.msg)
	b.bimpl = nil
}

// Stop stops and removes the bar.
func (b *virtualBar) Stop() {
	b.destroy()
}

// destroy destroys the virtualBar, after which it is no longer usable. On
// return, virtualBar.destroyed is true.
func (b *virtualBar) destroy() {
	if b == nil {
		return
	}

	b.destroyOnce.Do(func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.bimpl == nil {
			b.destroyed = true
			return
		}

		b.stopConcrete()
		b.destroyed = true
	})
}

var _ Bar = (*groupBar)(nil)

func (p *Progress) maybeGroupBar() *groupBar { //nolint:unused
	if p.groupBar != nil {
		p.groupBar.addOne()
		return p.groupBar
	}

	if len(p.bars) < groupBarThreshold {
		return nil
	}

	gb := &groupBar{
		p:           p,
		activeCount: &atomic.Int64{},
	}

	gb.init()
	gb.addOne()

	p.groupBar = gb
	return p.groupBar
}

// groupBar is a special Bar that groups multiple bars. Once groupBarThreshold
// number of bars are active, future bars are grouped into a single groupBar.
// We do this partially for UX, and partially because the mbp progress library
// slows down with lots of bars.
//
// NOTE: the groupBar mechanism  is not yet implemented.
type groupBar struct {
	p           *Progress
	activeCount *atomic.Int64
	vb          *virtualBar
	mu          sync.Mutex
}

func (mb *groupBar) refresh() {}

func (mb *groupBar) hide() {
}

func (mb *groupBar) show() {
}

func (mb *groupBar) init() {
	cfg := &barConfig{
		msg:   "Processing multiple",
		total: -1,
		style: spinnerStyle(mb.p.colors.Filler),
	}
	d := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current)
		s += " " + english.PluralWord(int(statistics.Current), "item", "items")
		return s
	})
	cfg.decorators = []decor.Decorator{colorize(d, mb.p.colors.Size)}

	vb := mb.p.createVirtualBar(cfg, nil)
	mb.vb = vb
}

func (mb *groupBar) addOne() { //nolint:unused
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.activeCount.Add(1)

	if mb.vb == nil {
		mb.init()
	}
}

func (mb *groupBar) Incr(n int) {
	mb.vb.Incr(n)
}

func (mb *groupBar) Stop() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.activeCount.Add(-1)
	if mb.activeCount.Load() > 0 {
		return
	}

	mb.vb.Stop()
	mb.vb = nil
}
