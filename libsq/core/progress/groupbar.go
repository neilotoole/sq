package progress

import (
	"github.com/dustin/go-humanize/english"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/vbauerster/mpb/v8/decor"
)

// groupBar groups multiple bars. Once groupBarThreshold number of bars is
// reached, further bars are grouped into a single groupBar. We do this
// partially for UX, and partially because the mbp progress library
// slows down with lots of bars.
//
// Note that groupBar doesn't need an internal mutex: it is not concurrently
// accessed.
type groupBar struct {
	// p is the Progress instance to which this groupBar belongs. It is always
	// non-nil if the groupBar is non-nil.
	p *Progress

	// vb is the groupBar's own virtualBar for rendering itself.
	vb *virtualBar
}

// newGroupBar returns a new groupBar (or nil) for Progress p. Note that only
// a single groupBar is created per Progress instance (at Progress creation).
func newGroupBar(p *Progress) *groupBar {
	if p == nil {
		return nil
	}

	msg := "Multiple operations"
	unit := "op"
	//vb := p.NewUnitCounter("Processing multiple", "items", )

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		msg:   msg,
		total: -1,
		style: spinnerStyle(p.colors.Filler),
	}

	fn := func(statistics decor.Statistics) string {
		s := humanize.Comma(statistics.Current)
		if unit != "" {
			s += " " + english.PluralWord(int(statistics.Current), unit, "")
		}
		return p.colors.Size.Sprint(s)
	}

	cfg.counterWidget = decor.Any(fn, p.align.counter)
	//cfg.counterWidget = colorize(cfg.counterWidget, p.colors.Size)

	//vb := p.createBar(cfg, false)

	gb := &groupBar{
		p:  p,
		vb: newVirtualBar(p, cfg, nil),
	}

	// We want the groupBar to be hidden initially. The refresh loop will make it
	// visible when appropriate.
	gb.vb.markHidden()
	return gb
}

// refresh refreshes the groupBar.
func (gb *groupBar) refresh(t time.Time) {
	if gb == nil || gb.vb == nil {
		return
	}

	select {
	case <-gb.p.stoppingCh:
		gb.vb.mu.Lock()
		gb.vb.stopConcrete()
		gb.vb.mu.Unlock()
	case <-gb.p.ctx.Done():
		gb.vb.mu.Lock()
		gb.vb.stopConcrete()
		gb.vb.mu.Unlock()
		return
	default:
	}

	if len(gb.p.activeInvisibleBars) == 0 {
		gb.vb.markHidden()
		gb.vb.mu.Lock()
		gb.vb.stopConcrete()
		gb.vb.mu.Unlock()
		return
	}

	gb.vb.Incr(gb.calculateIncr())
	gb.vb.markShown()

	gb.vb.mu.Lock()
	gb.vb.maybeShow(t)
	gb.vb.mu.Unlock()

	gb.vb.refresh(t)
}

func (gb *groupBar) calculateIncr() int {
	if gb == nil {
		return 0
	}

	var val int
	for _, vb := range gb.p.activeInvisibleBars {
		val += vb.groupIncrDelta()
	}

	return val
}

// destroy destroys the groupBar.
func (gb *groupBar) destroy() {
	if gb == nil {
		return
	}

	if gb.vb != nil {
		gb.vb.destroy()
	}

	gb.vb = nil
	gb.p.groupBar = nil
}
