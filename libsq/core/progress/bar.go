package progress

import (
	"sync"
	"sync/atomic"

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
	getDelayCh() <-chan struct{}
	ensureInit()
}

var _ Bar = (*virtualBar)(nil)

// virtualBar represents a single progress bar. The caller should invoke
// Incr as necessary to increment the bar's progress. When
// the bar is complete, the caller should invoke Bar.Stop. All
// methods are safe to call on a nil Bar.
type virtualBar struct {
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
	// after the render delay expires. In fact, even when the
	// bug is fixed, we may just stick with the lazy initialization
	// mechanism, as it allows us to set the render delay on a
	// per-bar basis, which is not possible with the mpb package.

	barInitOnce *sync.Once
	barInitFn   func()

	barStopOnce  *sync.Once
	barStoppedCh chan struct{}

	delayCh <-chan struct{}

	// incrStash holds the increment count until the
	// bar is fully initialized.
	incrStash *atomic.Int64

	// incrByCalls is the number of times IncrBy has been called.
	incrByCalls *atomic.Int64
}

func (b *virtualBar) ensureInit() {
	b.barInitOnce.Do(b.barInitFn)
}

func (b *virtualBar) getDelayCh() <-chan struct{} {
	return b.delayCh
}

// Incr increments progress by amount n. It is safe to
// call IncrBy on a nil Bar.
func (b *virtualBar) Incr(n int) {
	if b == nil {
		return
	}

	b.incrByCalls.Add(1)

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
		// The bar hasn't been initialized yet, so we stash
		// the increment count for later use.
		b.incrStash.Add(int64(n))
	}
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
	<-b.barStoppedCh
}

func (b *virtualBar) doStop() {
	if b == nil {
		return
	}

	b.barStopOnce.Do(func() {
		if b.bar == nil {
			close(b.barStoppedCh)
			return
		}

		// We *probably* only need to call b.bar.Abort() here?
		b.bar.SetTotal(-1, true)
		b.bar.Abort(true)
		b.bar.Wait()
		close(b.barStoppedCh)
	})
}

var _ Bar = nopBar{}

// nopBar is a no-op Bar.
type nopBar struct{}

func (b nopBar) getDelayCh() <-chan struct{} {
	return make(chan struct{})
}
func (b nopBar) ensureInit() {}
func (b nopBar) Incr(_ int)  {}
func (b nopBar) Stop()       {}

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
		delayCh:     make(chan struct{}),
	}
	close(mega.delayCh)

	mega.init()
	mega.addOne()

	p.megaBar = mega
	return p.megaBar
}

type megaBar struct {
	p           *Progress
	activeCount *atomic.Int64
	vb          *virtualBar
	delayCh     chan struct{}
	mu          sync.Mutex
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

func (mb *megaBar) getDelayCh() <-chan struct{} {
	return mb.delayCh
}

func (mb *megaBar) ensureInit() {
}
