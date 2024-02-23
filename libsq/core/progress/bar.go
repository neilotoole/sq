package progress

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// Bar represents a single progress bar. The caller should invoke Incr as
// necessary to increment the bar's progress. When the bar is complete, the
// caller should invoke Bar.Stop.
type Bar interface {
	Incr(n int)
	Stop()
	hide()
	show()
	refresh()
}

var _ Bar = (*virtualBar)(nil)

// virtualBar represents a single progress bar. The caller should invoke
// Incr as necessary to increment the bar's progress. When
// the bar is complete, the caller should invoke Bar.Stop. All
// methods are safe to call on a nil Bar.
type virtualBar struct {
	// delayUntil is a checkpoint before which the virtualBar isn't shown. It's
	// basically a render delay.
	delayUntil time.Time

	// incrLastSentTime is when incrLastSentVal was last sent to bimpl.
	incrLastSentTime time.Time

	// cfg is the bar's configuration. It is preserved so that the
	cfg *barConfig

	// bimpl is the concrete mpb.Bar impl. While the virtualBar is hidden, or
	// stopped, bimpl is nil. While the virtualBar is shown, bimpl is non-nil.
	bimpl *mpb.Bar

	// p is the virtualBar's parent Progress.
	p *Progress

	barStopOnce  *sync.Once
	barStoppedCh chan struct{}

	// incrTotal holds the total value of increment values passed to Incr.
	incrTotal *atomic.Int64

	// incrByCalls is a count of virtualBar.Incr invocations. It's used for
	// logging stats.
	incrByCalls *atomic.Int64

	// incrLastSentVal is the last value sent to bimpl.
	incrLastSentVal int64

	mu sync.Mutex

	// shouldShow is true if the virtualBar is supposed to be shown at this time.
	shouldShow bool
}

func HideBar(b Bar) {
	if b == nil {
		return
	}

	b.hide()
}

func ShowBar(b Bar) {
	if b == nil {
		return
	}

	b.show()
}

func (b *virtualBar) refresh() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.shouldShow {
		if b.bimpl != nil {
			b.doHide()
		}
		return
	}

	if b.bimpl == nil {
		b.doShow()
		return
	}

	b.maybeSendIncr()
}

func (b *virtualBar) maybeSendIncr() {
	if b == nil || b.bimpl == nil || !b.shouldShow {
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

func (b *virtualBar) hide() {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.doHide()
}

func (b *virtualBar) show() {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.doShow()
}

func (b *virtualBar) doShow() {
	if b == nil {
		return
	}

	b.shouldShow = true
	if b.bimpl != nil {
		return
	}

	if !time.Now().After(b.delayUntil) {
		return
	}

	b.start()
}

func (b *virtualBar) start() {
	defer func() {
		if r := recover(); r != nil {
			// If we panic here, it's likely because the progress has already
			// been stopped.
			err := errz.Errorf("progress: new bar: %v", r)
			lg.FromContext(b.p.ctx).Warn("Caught panic in progress.barFromConfig", lga.Err, err)
		}
	}()

	pBar := b.p.pc.New(b.cfg.total,
		b.cfg.style,
		mpb.BarWidth(barWidth),
		mpb.PrependDecorators(
			colorize(decor.Name(b.cfg.msg, decor.WCSyncWidthR), b.p.colors.Message),
		),
		mpb.AppendDecorators(b.cfg.decorators...),
		mpb.BarRemoveOnComplete(),
	)
	b.bimpl = pBar

	// send the total value to the bar.
	total := b.incrTotal.Load()
	b.incrLastSentVal = total
	b.incrLastSentTime = time.Now()
	b.bimpl.IncrBy(int(total))
}

// Incr increments progress by amount n. It is safe to
// call IncrBy on a nil Bar.
func (b *virtualBar) Incr(n int) {
	if b == nil {
		return
	}

	b.incrTotal.Add(int64(n))
	b.incrByCalls.Add(1)
}

// Stop stops and removes the bar. It is safe to call Stop on a nil Bar,
// or to call Stop multiple times.
func (b *virtualBar) Stop() {
	if b == nil {
		return
	}

	b.p.mu.Lock()
	defer b.p.mu.Unlock()

	b.doStop()
	// b.doHide()
	<-b.barStoppedCh
}

func (b *virtualBar) doHide() {
	if b == nil {
		return
	}

	b.shouldShow = false
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

func (b *virtualBar) doStop() {
	if b == nil {
		return
	}

	b.barStopOnce.Do(func() {
		if b.bimpl == nil {
			close(b.barStoppedCh)
			return
		}

		b.doHide()
		close(b.barStoppedCh)
	})
}

var _ Bar = nopBar{}

// nopBar is a no-op Bar.
type nopBar struct{}

func (b nopBar) refresh() {}

func (b nopBar) hide() {}

func (b nopBar) show() {}

func (b nopBar) Incr(_ int) {}
func (b nopBar) Stop()      {}

var _ Bar = (*megaBar)(nil)

func (p *Progress) maybeMegaBar() *megaBar { //nolint:unused
	if p.megaBar != nil {
		p.megaBar.addOne()
		return p.megaBar
	}

	if len(p.bars) < maxActiveBars {
		return nil
	}

	mega := &megaBar{
		p:           p,
		activeCount: &atomic.Int64{},
	}

	mega.init()
	mega.addOne()

	p.megaBar = mega
	return p.megaBar
}

type megaBar struct {
	p           *Progress
	activeCount *atomic.Int64
	vb          *virtualBar
	mu          sync.Mutex
}

func (mb *megaBar) refresh() {}

func (mb *megaBar) hide() {
}

func (mb *megaBar) show() {
}

func (mb *megaBar) init() {
	cfg := &barConfig{
		msg:   "Mega mega mega",
		total: -1,
		style: spinnerStyle(mb.p.colors.Filler),
	}
	d := decor.Any(func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current)
		s += " " + english.PluralWord(int(statistics.Current), "thing", "things")
		return s
	})
	cfg.decorators = []decor.Decorator{colorize(d, mb.p.colors.Size)}

	vb := mb.p.createVirtualBar(cfg, nil)
	mb.vb = vb
}

func (mb *megaBar) addOne() { //nolint:unused
	mb.mu.Lock()
	defer mb.mu.Unlock()
	mb.activeCount.Add(1)

	if mb.vb == nil {
		mb.init()
	}
}

func (mb *megaBar) Incr(n int) {
	mb.vb.Incr(n)
}

func (mb *megaBar) Stop() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.activeCount.Add(-1)
	if mb.activeCount.Load() > 0 {
		return
	}

	mb.vb.Stop()
	mb.vb = nil
}
