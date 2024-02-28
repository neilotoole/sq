package progress

import (
	"sync"
	"sync/atomic"
	"time"

	mpb "github.com/vbauerster/mpb/v8"
)

// newVirtualBar returns a new virtualBar (or nil). Generally speaking, callers
// should use Progress.createBar instead of calling newVirtualBar directly.
//
// Note that the returned virtualBar is NOT automatically shown, nor is it
// automatically added to Progress.allBars.
//
// The caller should hold Progress.mu.
func newVirtualBar(p *Progress, cfg *barConfig, opts []BarOpt) *virtualBar {
	if p == nil {
		return nil
	}

	select {
	case <-p.destroyedCh:
		return nil
	default:
	}

	if cfg.total < 0 {
		cfg.total = 0
	}

	for _, opt := range opts {
		if opt != nil {
			opt.apply(p, cfg)
		}
	}

	if cfg.counterWidget == nil {
		cfg.counterWidget = nopWidget(p, p.align.counter)
	}
	if cfg.timerWidget == nil {
		cfg.timerWidget = nopWidget(p, p.align.timer)
	}
	if cfg.percentWidget == nil {
		cfg.percentWidget = nopWidget(p, p.align.percent)
	}
	if cfg.memoryWidget == nil {
		cfg.memoryWidget = nopWidget(p, p.align.mem)
	}

	vb := &virtualBar{
		p:           p,
		incrByCalls: &atomic.Int64{},
		incrTotal:   &atomic.Int64{},
		destroyOnce: &sync.Once{},
		notBefore:   time.Now().Add(p.renderDelay),
		cfg:         cfg,
	}

	return vb
}

var _ Bar = (*virtualBar)(nil)

// virtualBar is the main implementation of Bar. It is a "virtual bar" in the
// sense that it is not a concrete mpb.Bar, but rather an abstraction that
// may create and destroy a concrete mpb.Bar as necessary.
type virtualBar struct {
	// notBefore is a checkpoint before which the virtualBar isn't shown. It's
	// basically a render delay for the virtualBar.
	notBefore time.Time

	// incrLastSentTime is when incrLastSentVal was last sent to bimpl.
	incrLastSentTime time.Time

	// cfg is the bar's configuration. It is preserved so that the concrete bimpl
	// can be created, destroyed, and recreated as needed.
	cfg *barConfig

	// bimpl is the concrete mpb.Bar impl. While the virtualBar is hidden, or
	// destroyed, bimpl is nil. While the virtualBar is shown, bimpl is non-nil.
	bimpl *mpb.Bar

	// p is the virtualBar's parent Progress. It is always non-nil if the
	// virtualBar is non-nil.
	p *Progress

	// incrTotal holds the cumulative value of virtualBar.Incr.
	incrTotal *atomic.Int64

	// incrByCalls is a count of virtualBar.Incr invocations. It's used for
	// logging stats.
	incrByCalls *atomic.Int64

	// destroyOnce is used within virtualBar.destroy.
	destroyOnce *sync.Once

	// incrLastSentVal is the most recent increment total value sent to bimpl.
	incrLastSentVal int64

	// groupLastSentVal is the most recent increment value consumed by
	// Progress.groupBar.
	groupLastSentVal int64

	// mu guards the virtualBar's fields.
	mu sync.Mutex

	// destroyed is set to true by virtualBar.destroy
	destroyed bool

	// wantShow is true if we want the virtualBar to be shown. However, even if
	// true, the bar may not be shown if notBefore hasn't been reached.
	wantShow bool
}

// isRenderable returns true if the virtualBar is renderable at time t. It will
// not be renderable if b.notBefore hasn't passed, or if b is nil or destroyed.
func (vb *virtualBar) isRenderable(t time.Time) bool {
	if vb == nil || t.Before(vb.notBefore) {
		return false
	}

	vb.mu.Lock()
	defer vb.mu.Unlock()
	return !vb.destroyed
}

// Incr tracks the bar's increment value. The tracked value may be used by
// virtualBar.refresh to update the concrete mpb.Bar.
func (vb *virtualBar) Incr(n int) {
	if vb == nil {
		return
	}

	vb.incrTotal.Add(int64(n))
	vb.incrByCalls.Add(1)
}

// refresh is called by the Progress's refresh goroutine, potentially creating,
// incrementing, or destroying virtualBar.bimpl.
func (vb *virtualBar) refresh(t time.Time) {
	if vb == nil {
		return
	}

	vb.mu.Lock()
	defer vb.mu.Unlock()
	if !vb.p.life.alive() {
		vb.stopConcrete()
		return
	}

	switch {
	case vb.destroyed:
		return
	case !vb.wantShow:
		// We don't want to show the bar.
		if vb.bimpl != nil {
			// If the concrete impl is present, we need to nuke it.
			vb.stopConcrete()
		}
		return
	case vb.bimpl == nil:
		// We want to show the bar, but the concrete impl isn't present.
		vb.maybeShow(t)
	default:
	}

	vb.maybeSendConcreteIncr()
}

// maybeSendConcreteIncr updates concrete mpb.Bar with the current increment,
// if appropriate. The increment won't be sent if the concrete bar doesn't
// exist, or if b is destroyed, etc.
//
// The caller must hold the virtualBar.mu lock.
func (vb *virtualBar) maybeSendConcreteIncr() {
	if vb == nil || !vb.wantShow || vb.destroyed || vb.bimpl == nil {
		return
	}

	total := vb.incrTotal.Load()
	amount := total - vb.incrLastSentVal
	if amount == 0 {
		return
	}

	vb.bimpl.IncrBy(int(amount))
	vb.incrLastSentVal = total
	vb.incrLastSentTime = time.Now()
}

// groupIncrDelta returns the increment delta to be consumed by
// Progress.groupBar, if vb is part of a groupBar.
func (vb *virtualBar) groupIncrDelta() int {
	if vb == nil {
		return 0
	}

	vb.mu.Lock()
	defer vb.mu.Unlock()

	if vb.destroyed {
		return 0
	}

	delta := vb.incrTotal.Load() - vb.incrLastSentVal - vb.groupLastSentVal
	vb.groupLastSentVal += delta
	return int(delta)
}

// markShown implements Bar.markShown.
func (vb *virtualBar) markShown() {
	if vb == nil {
		return
	}

	vb.mu.Lock()
	vb.wantShow = true
	vb.mu.Unlock()
}

// maybeShow maybe causes the virtualBar to be rendered, if appropriate. It may
// instantiate a concrete mpb.Bar via virtualBar.startConcrete.
//
// The caller must hold the virtualBar.mu lock.
func (vb *virtualBar) maybeShow(t time.Time) {
	if vb == nil || vb.destroyed {
		return
	}

	vb.wantShow = true
	if vb.bimpl != nil {
		return
	}

	if !t.After(vb.notBefore) {
		return
	}

	vb.startConcrete()
}

// startConcrete start's the virtualBar's concrete mpb.Bar.
//
// The caller must hold the virtualBar.mu lock.
func (vb *virtualBar) startConcrete() {
	if vb == nil || vb.p == nil {
		return
	}

	if vb.destroyed {
		return
	}

	select {
	case <-vb.p.life.dyingCh:
		return
	case <-vb.p.ctx.Done():
		return
	default:
	}

	// Recover on any interaction with mpb.
	defer func() { _ = recover() }()

	vb.bimpl = vb.p.life.pc.New(vb.cfg.total,
		vb.cfg.style,
		mpb.BarWidth(barWidth),
		mpb.PrependDecorators(vb.cfg.msgWidget),
		mpb.AppendDecorators(vb.cfg.counterWidget, vb.cfg.percentWidget, vb.cfg.timerWidget, vb.cfg.memoryWidget),
		mpb.BarRemoveOnComplete(),
	)

	// Send the total value to the bar.
	total := vb.incrTotal.Load()
	vb.incrLastSentVal = total
	vb.incrLastSentTime = time.Now()
	vb.bimpl.IncrBy(int(total))
}

// markHidden marks the virtualBar as hidden. On the alive refresh, b's concrete
// bar may be removed.
func (vb *virtualBar) markHidden() {
	if vb == nil {
		return
	}

	vb.mu.Lock()
	vb.wantShow = false
	vb.mu.Unlock()
}

func (vb *virtualBar) hide() {
	if vb == nil {
		return
	}

	vb.mu.Lock()
	defer vb.mu.Unlock()
	vb.wantShow = false
	vb.stopConcrete()
}

// stopConcrete stops the concrete virtualBar.bimpl.
//
// The caller must hold the virtualBar.mu lock.
func (vb *virtualBar) stopConcrete() {
	if vb == nil {
		return
	}

	vb.wantShow = false
	if vb.bimpl == nil {
		return
	}

	defer func() {
		vb.bimpl = nil
		// Recover on any interaction with mpb.
		_ = recover()
	}()

	// We *probably* only need to call b.bar.Abort() here?
	vb.bimpl.SetTotal(-1, true)
	vb.bimpl.Abort(true)
	vb.bimpl.Wait()
	vb.bimpl = nil
}

// Stop stops and removes the bar.
func (vb *virtualBar) Stop() {
	if vb == nil {
		return
	}

	vb.destroy()
	vb.p.forgetBar(vb)
}

// destroy destroys the virtualBar, after which it is no longer usable. On
// return, virtualBar.destroyed is true.
func (vb *virtualBar) destroy() {
	if vb == nil {
		return
	}

	vb.destroyOnce.Do(func() {
		vb.mu.Lock()
		defer vb.mu.Unlock()
		if vb.bimpl == nil {
			vb.destroyed = true
			return
		}

		vb.stopConcrete()
		vb.destroyed = true
	})
}
