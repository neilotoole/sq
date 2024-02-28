package progress

import (
	"fmt"
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/vbauerster/mpb/v8/decor"
)

// groupBar groups multiple bars. Once Progress.groupThreshold number of bars is
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

	p.mu.Lock()
	defer p.mu.Unlock()

	cfg := &barConfig{
		total: -1,
		style: spinnerStyle(p.colors.Filler),
	}

	fn := func(statistics decor.Statistics) string {
		switch statistics.Current {
		case 0:
			return ""
		case 1:
			return "1 item"
		default:
			return humanize.Comma(statistics.Current) + " items"
		}
	}
	cfg.counterWidget = colorize(decor.Any(fn, p.align.counter), p.colors.Size)

	// Add a timer widget.
	OptTimer.apply(p, cfg)

	// The groupBar's message widget is dynamic (incorporating the count of active
	// invisible bars).
	cfg.msgWidget = colorize(
		decor.Any(
			func(statistics decor.Statistics) string {
				m := fmt.Sprintf("More operations (%d)", len(p.activeInvisibleBars))
				return msgWidth(m)
			},
			p.align.msg,
		),
		p.colors.Message,
	)

	gb := &groupBar{
		p:  p,
		vb: newVirtualBar(p, cfg, nil),
	}

	// We want the groupBar to be hidden initially. The state refresh loop will
	// make it visible when appropriate.
	gb.vb.markHidden()
	return gb
}

// refresh refreshes the groupBar.
func (gb *groupBar) refresh(t time.Time) {
	if gb == nil || gb.vb == nil {
		return
	}

	if !gb.p.life.alive() || len(gb.p.activeInvisibleBars) == 0 {
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

func (gb *groupBar) hide() {
	if gb == nil || gb.vb == nil {
		return
	}

	gb.vb.hide()
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

// destroy destroys the groupBar. // FIXME: Do we need this?
func (gb *groupBar) destroy() { //nolint:unused
	if gb == nil {
		return
	}

	if gb.vb != nil {
		gb.vb.destroy()
	}

	gb.vb = nil
	gb.p.groupBar = nil
}
