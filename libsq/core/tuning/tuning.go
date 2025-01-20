// Package tuning contains tuning options.
package tuning

import (
	"time"

	"github.com/neilotoole/sq/libsq/core/datasize"

	"github.com/neilotoole/sq/libsq/core/options"
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

var OptRecBufSize = options.NewInt(
	"tuning.record-buffer",
	nil,
	1024,
	"Size of record buffer",
	`Controls the size of the channel for buffering records.`,
	options.TagTuning,
)

var OptFlushThreshold = datasize.NewOpt(
	"tuning.output-flush-threshold",
	nil,
	datasize.MustParseString("1000B"),
	"Output writer buffer flush threshold.",
	`Size after which output writers should flush any internal buffer.
Generally, it is not necessary to fiddle this knob.

Use units B, KB, MB, GB, etc. For example, 64KB, or 10MB. If no unit specified,
bytes are assumed.`,
	options.TagTuning,
)

var OptBufSpillLimit = datasize.NewOpt(
	"tuning.buffer-spill-limit",
	nil,
	datasize.MustParseString("1MB"),
	"Buffer swap file memory limit",
	`Size after which in-memory temp buffers spill to disk.

Use units B, KB, MB, GB, etc. For example, 64KB, or 10MB. If no unit specified,
bytes are assumed.`,
	options.TagTuning,
)
