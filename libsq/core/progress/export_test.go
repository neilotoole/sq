package progress

import "time"

// This file exposes internals to the progress_test package (white-box tests)
// so that the lifecycle, refresh, and group-bar machinery can be driven
// deterministically without relying on real-time ticker behavior.

// Export pure formatter functions for direct testing.
var (
	GroupBarCounterText  = groupBarCounterText
	UnitCounterText      = unitCounterText
	UnitTotalCounterText = unitTotalCounterText
	TimeoutText          = timeoutText
	MsgWidth             = msgWidth
)

// MsgMaxLen is the maximum message width, exported for tests.
const MsgMaxLen = msgMaxLen

// BarCount returns the number of non-destroyed bars tracked by p.
func (p *Progress) BarCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.allBars)
}

// EnsureLife blocks until the Progress has a live lifecycle, or returns false
// if one couldn't be established within a reasonable number of attempts. It's
// used by tests that need the refresh machinery to be active.
func (p *Progress) EnsureLife() bool {
	for range 200 {
		p.mu.Lock()
		life := p.life
		p.mu.Unlock()
		if life != nil && life.alive() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// ForceRefresh synchronously runs one state-refresh iteration at time t,
// bypassing the ticker. It requires a live lifecycle (see EnsureLife). It
// returns true if the refresh loop signaled that it should stop.
func (p *Progress) ForceRefresh(t time.Time) (stop bool) {
	p.mu.Lock()
	life := p.life
	p.mu.Unlock()
	if life == nil {
		return true
	}
	return life.refreshState(t)
}

// ActiveBarCounts returns the number of visible and invisible (grouped) bars as
// recorded by the most recent refresh.
func (p *Progress) ActiveBarCounts() (visible, invisible int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.activeVisibleBars), len(p.activeInvisibleBars)
}

// SetNotBefore sets the Progress's notBefore checkpoint, allowing tests to make
// bars immediately renderable.
func (p *Progress) SetNotBefore(t time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.notBefore = t
}

// BarNotBefore exposes a bar's render-delay checkpoint for tests.
func BarNotBefore(b Bar) (time.Time, bool) {
	vb, ok := b.(*virtualBar)
	if !ok {
		return time.Time{}, false
	}
	vb.mu.Lock()
	defer vb.mu.Unlock()
	return vb.notBefore, true
}

// SetBarNotBefore overrides a bar's render-delay checkpoint, making it
// immediately renderable when set to the past.
func SetBarNotBefore(b Bar, t time.Time) bool {
	vb, ok := b.(*virtualBar)
	if !ok {
		return false
	}
	vb.mu.Lock()
	defer vb.mu.Unlock()
	vb.notBefore = t
	return true
}

// GroupIncrDelta exposes virtualBar.groupIncrDelta for tests.
func GroupIncrDelta(b Bar) (int, bool) {
	vb, ok := b.(*virtualBar)
	if !ok {
		return 0, false
	}
	return vb.groupIncrDelta(), true
}

// CalculateGroupIncr exposes the group bar's aggregate increment calculation.
func (p *Progress) CalculateGroupIncr() int {
	return p.groupBar.calculateIncr()
}
