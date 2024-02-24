package progress

// Development of this package has been a bit of an ordeal. It has taken many
// iterations to get to a point where it's mostly stable and performant. This
// debug.go file contains dev/debug functionality. When we're completely happy
// with the package, this stuff can be deleted.

import (
	"context"
	"time"

	"github.com/samber/lo"

	"github.com/neilotoole/sq/libsq/core/options"
)

// OptDebugSleep configures DebugSleep. It should be removed when the
// progress impl is stable.
var OptDebugSleep = options.NewDuration(
	"debug.progress.sleep",
	nil,
	0,
	"DEBUG: Sleep during operations to facilitate testing progress bars",
	`DEBUG: Sleep during operations to facilitate testing progress bars.`,
)

// OptDebugForce forces instantiation of progress bars, even if stderr is not a
// terminal. It should be removed when the progress impl is stable.
var OptDebugForce = options.NewBool(
	"debug.progress.force",
	nil,
	false,
	"DEBUG: Always render progress bars",
	`DEBUG: Always render progress bars, even when stderr is not a terminal, or
progress is not enabled. This is useful for testing the progress impl.`,
)

// DebugSleep sleeps for a period of time to facilitate testing the
// progress impl. It uses the value from OptDebugSleep. This function
// (and OptDebugSleep) should be removed when the progress impl is
// stable.
func DebugSleep(ctx context.Context) {
	sleep := OptDebugSleep.Get(options.FromContext(ctx))
	if sleep > 0 {
		time.Sleep(sleep)
	}
}

// ShowBar shows bar b. This is a temporary function for testing.
// FIXME: Delete ShowBar.
func ShowBar(b Bar) {
	if lo.IsNil(b) {
		return
	}

	b.show()
}

// HideBar hides bar b. This is a temporary function for testing.
// FIXME: Delete HideBar.
func HideBar(b Bar) {
	if lo.IsNil(b) {
		return
	}

	b.hide()
}

// Hide hides the Progress.
//
// FIXME: Hide is a temporary function for testing. It should be removed.
func Hide(p *Progress) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, bar := range p.bars {
		bar.hide()
	}
}

// Show shows the Progress.
//
// FIXME: Show is a temporary function for testing. It should be removed.
func Show(p *Progress) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, bar := range p.bars {
		bar.show()
	}
}
