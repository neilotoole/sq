package progress

// Development of this package has been a bit of an ordeal. It has taken many
// iterations to get to a point where it's mostly stable and performant. This
// debug.go file contains dev/debug functionality. When we're completely happy
// with the package, this stuff can be deleted.

import (
	"github.com/samber/lo"
)

// ShowBar shows bar b. This is a temporary function for testing.
// FIXME: Delete ShowBar.
func ShowBar(b Bar) {
	if lo.IsNil(b) {
		return
	}

	b.markShown()
}

// HideBar hides bar b. This is a temporary function for testing.
// FIXME: Delete HideBar.
func HideBar(b Bar) {
	if lo.IsNil(b) {
		return
	}

	b.markHidden()
}
