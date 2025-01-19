// Package tuning contains tuning options.
package tuning

import (
	"bufio"
	"context"
	"time"

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

var OptFlushThreshold = options.NewInt(
	"tuning.output-flush-threshold",
	nil,
	1000,
	"Output writer buffer flush threshold in bytes",
	`Size in bytes after which output writers should flush any internal buffer.
Generally, it is not necessary to fiddle this knob.`,
	options.TagTuning,
)

var OptBufMemLimit = options.NewInt(
	"tuning.buffer-mem-limit",
	nil,
	1000*1000, // 1MB
	"Buffer swap file memory limit",
	`Size in bytes after which in-memory temp buffers overflow to disk.`,
	options.TagTuning,
)

var OptScanTokenBufLimit = options.NewInt(
	"tuning.scan-token-buffer-limit",
	nil,
	1000*1000, // 1MB
	"Scan token buffer limit",
	`Size in bytes of the buffer used for scanning tokens.`,
	options.TagTuning,
)

// ConfigureBufioScanner configures the bufio.Scanner sc with the buffer
// size taken from OptScanTokenBufLimit. The configured scanner is returned
// for fluency.
func ConfigureBufioScanner(ctx context.Context, sc *bufio.Scanner) *bufio.Scanner {
	sc.Buffer(make([]byte, 1024*64), OptScanTokenBufLimit.Get(options.FromContext(ctx)))
	return sc
}
