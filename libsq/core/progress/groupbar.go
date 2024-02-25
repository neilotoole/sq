package progress

import (
	"sync"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/vbauerster/mpb/v8/decor"
)

// groupBar groups multiple bars. Once groupBarThreshold number of bars is
// reached, further bars are grouped into a single groupBar. We do this
// partially for UX, and partially because the mbp progress library
// slows down with lots of bars.
type groupBar struct {
	p *Progress

	// vb is the groupBar's own virtualBar for rendering itself.
	vb *virtualBar
	mu sync.Mutex
}

func newGroupBar(p *Progress) *groupBar {
	if p == nil {
		return nil
	}
	cfg := &barConfig{
		msg:   "Processing multiple",
		total: -1,
		style: spinnerStyle(p.colors.Filler),
	}
	d := decor.Any(func(statistics decor.Statistics) string {
		if statistics.Current <= 0 {
			return ""
		}

		s := humanize.Comma(statistics.Current)
		s += " " + english.PluralWord(int(statistics.Current), "item", "items")
		return s
	})
	cfg.decorators = []decor.Decorator{colorize(d, p.colors.Size)}

	gb := &groupBar{
		p:  p,
		vb: newVirtualBar(p, cfg, nil),
	}
	gb.vb.markHidden()
	return gb
}

func (gb *groupBar) refresh(t time.Time) {
	if len(gb.p.activeInvisibleBars) == 0 {
		gb.vb.markHidden()
		return
	}

	gb.vb.Incr(gb.calculateIncr())
	gb.vb.markShown()
	gb.vb.maybeShow(t)
	gb.vb.refresh(t)
}

func (gb *groupBar) calculateIncr() int {
	var val int
	for _, vb := range gb.p.activeInvisibleBars {
		val += vb.groupIncrDelta()
	}

	return val
}

func (gb *groupBar) destroy() {
	if gb == nil {
		return
	}
	gb.mu.Lock()
	defer gb.mu.Unlock()

	if gb.vb != nil {
		gb.vb.destroy()
	}
}
