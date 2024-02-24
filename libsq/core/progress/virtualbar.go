package progress

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/samber/lo"

	mpb "github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
)

// newVirtualBar returns a new virtualBar (or nil). It must only be called
// from inside the Progress mutex. Generally speaking, callers should use
// Progress.barFromConfig instead of calling newVirtualBar directly.
//
// Note that the returned virtualBar is NOT automatically shown, nor is it
// automatically added to Progress.bars.
func newVirtualBar(p *Progress, cfg *barConfig, opts []Opt) *virtualBar {
	if p == nil {
		return nil
	}

	cfg.decorators = lo.WithoutEmpty(cfg.decorators)

	select {
	case <-p.destroyedCh:
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

	return b
}

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

func (b *virtualBar) isRendered(t time.Time) bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.wantShow && t.After(b.notBefore)
}

func (b *virtualBar) isRenderable(t time.Time) bool {
	if b == nil {
		return false
	}

	return t.After(b.notBefore)
}

// Incr increments the progress bar by amount n.
func (b *virtualBar) Incr(n int) {
	if b == nil {
		return
	}

	b.incrTotal.Add(int64(n))
	b.incrByCalls.Add(1)
}

func (b *virtualBar) refresh(t time.Time) {
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
		b.maybeShow(t)
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

	b.wantShow = true

	// b.maybeShow(time.Now())
}

// maybeShow must be called inside b's mutex.
func (b *virtualBar) maybeShow(t time.Time) {
	if b == nil || b.destroyed {
		return
	}

	b.wantShow = true
	if b.bimpl != nil {
		return
	}

	if !t.After(b.notBefore) {
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
	// b.stopConcrete()
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
	if b == nil {
		return
	}

	b.destroy()
	b.p.delistBar(b)
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
