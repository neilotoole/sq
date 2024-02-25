// Package debugz contains functionality for debugging. At some point it should
// be possible to delete this package.
package debugz

import (
	"context"
	"time"

	"github.com/neilotoole/sq/libsq/core/options"
)

// OptProgressDebugSleep configures DebugSleep. It should be removed when the
// progress impl is stable.
var OptProgressDebugSleep = options.NewDuration(
	"debug.progress.sleep",
	nil,
	0,
	"DEBUG: Sleep during operations to facilitate testing progress bars",
	`DEBUG: Sleep during operations to facilitate testing progress bars.`,
)

// OptProgressDebugForce forces instantiation of progress bars, even if stderr is not a
// terminal. It should be removed when the progress impl is stable.
var OptProgressDebugForce = options.NewBool(
	"debug.progress.force",
	nil,
	false,
	"DEBUG: Always render progress bars",
	`DEBUG: Always render progress bars, even when stderr is not a terminal, or
progress is not enabled. This is useful for testing the progress impl.`,
)

// DebugSleep sleeps for a period of time to facilitate testing the
// progress impl. It uses the value from OptProgressDebugSleep. This function
// (and OptProgressDebugSleep) should be removed when the progress impl is
// stable.
func DebugSleep(ctx context.Context) {
	sleep := OptProgressDebugSleep.Get(options.FromContext(ctx))
	if sleep > 0 {
		time.Sleep(sleep)
	}
}
