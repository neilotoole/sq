// Package tuning contains tuning options.
package tuning

import (
	"github.com/neilotoole/sq/libsq/core/options"
	"time"
)

var OptMaxRetryInterval = options.NewDuration(
	"retry.max-interval",
	nil,
	time.Second*3,
	"Max interval between retries",
	`The maximum interval to wait between retries. If an operation is
retryable (for example, if the DB has too many clients), repeated retry
operations back off, typically using a Fibonacci backoff.`,
	options.TagSource,
)

var OptErrgroupLimit = options.NewInt(
	"tuning.errgroup-limit",
	nil,
	16,
	"Max goroutines in any one errgroup",
	`Controls the maximum number of goroutines that can be spawned by an errgroup.
Note that this is the limit for any one errgroup, but not a ceiling on the total
number of goroutines spawned, as some errgroups may themselves start an
errgroup.

This knob is primarily for internal use. Ultimately it should go away in favor
of dynamic errgroup limit setting based on availability of additional DB conns,
etc.`,
	options.TagTuning,
)

var OptRecChanSize = options.NewInt(
	"tuning.record-buffer",
	nil,
	1024,
	"Size of record buffer",
	`Controls the size of the channel for buffering records.`,
	options.TagTuning,
)

var OptFlushThreshold = options.NewInt(
	"tuning.flush-threshold",
	nil,
	1000,
	"Output writer buffer flush threshold in bytes",
	`Size in bytes after which output writers should flush any internal buffer.
Generally, it is not necessary to fiddle this knob.`,
	options.TagTuning,
)
