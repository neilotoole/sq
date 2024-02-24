package progress

import (
	"sync"
	"time"

	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/dustin/go-humanize"
	"github.com/dustin/go-humanize/english"
	"github.com/vbauerster/mpb/v8/decor"
)

// groupBar is a special Bar that groups multiple bars. Once groupBarThreshold
// number of bars are active, future bars are grouped into a single groupBar.
// We do this partially for UX, and partially because the mbp progress library
// slows down with lots of bars.
//
// NOTE: the groupBar mechanism  is not yet implemented.
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
		s := humanize.Comma(statistics.Current)
		s += " " + english.PluralWord(int(statistics.Current), "item", "items")
		return s
	})
	cfg.decorators = []decor.Decorator{colorize(d, p.colors.Size)}

	gb := &groupBar{
		p:  p,
		vb: newVirtualBar(p, cfg, nil),
	}
	gb.vb.hide()
	return gb
}

func (gb *groupBar) isRendered(t time.Time) bool {
	if gb == nil || gb.p == nil {
		return false
	}
	return len(gb.p.activeInvisibleBars) > 0
}

func (gb *groupBar) refresh(t time.Time) {
	if !gb.isRendered(t) {
		gb.vb.hide()
		return
	}

	groupIncr := gb.calculateIncr()
	gb.vb.Incr(groupIncr) // FIXME: calculate real value
	gb.vb.show()
	gb.vb.maybeShow(t)
	gb.vb.refresh(t)
}

func (gb *groupBar) calculateIncr() int {
	var val int
	for _, vb := range gb.p.activeInvisibleBars {
		val += vb.getGroupIncr()
	}

	lg.FromContext(gb.p.ctx).Debug("groupBar.calculateIncr", lga.Val, val)
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

// var _ Bar = (*groupBar)(nil)
const enableGroupBar = false

//func (p *Progress) countVisibleBars(t time.Time) (count int) {
//	if p == nil {
//		return count
//	}
//
//	for _, vb := range p.allBars {
//		if vb == p.groupBar.vb {
//			continue
//		}
//		if vb.isRendered(t) {
//			count++
//		}
//	}
//
//	return count
//}
//
//func (p *Progress) needGroupBar() bool {
//	if !enableGroupBar {
//		return false
//	}
//
//	if p == nil {
//		return false
//	}
//
//	if p.groupBar != nil {
//		return true
//	}
//
//	if len(p.allBars) < groupBarThreshold {
//		return false
//	}
//
//	return true
//}
