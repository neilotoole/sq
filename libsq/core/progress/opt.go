package progress

import (
	"time"

	"github.com/neilotoole/sq/libsq/core/options"
)

var OptEnable = options.NewBool(
	"progress",
	&options.Flag{
		Name:   "no-progress",
		Invert: true,
		Usage:  "Don't show progress bar",
	},
	true,
	"Show progress bar for long-running operations",
	`Show progress bar for long-running operations.`,
	options.TagOutput,
)

var OptDelay = options.NewDuration(
	"progress.delay",
	nil,
	time.Second*2,
	"Progress bar render delay",
	`Delay before showing a progress bar.`,
)
