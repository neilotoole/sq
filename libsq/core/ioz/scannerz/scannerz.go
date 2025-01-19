// Package scannerz contains functionality for scanning data, most specifically
// for working with bufio.Scanner.
package scannerz

import (
	"bufio"
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq/core/datasize"

	"github.com/neilotoole/sq/libsq/core/options"
)

// OptScanBufLimit is an Opt for configuring the buffer size of a bufio.Scanner
// returned from scannerz.NewScanner.
var OptScanBufLimit = datasize.NewOpt(
	"tuning.scan-buffer-limit",
	nil,
	datasize.MustParseString("8MB"),
	"Scan token buffer limit",
	`Size of the buffer used for scanning tokens.

Use units B, KB, MB, GB, etc. For example, 64KB, or 10MB. If no unit specified,
bytes are assumed.`,
	options.TagTuning,
)

// NewScanner returns a new bufio.Scanner configured via OptScanBufLimit
// set on ctx.
func NewScanner(ctx context.Context, r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)

	opt := OptScanBufLimit.Get(options.FromContext(ctx))
	limit := opt.Bytes()
	initialBufSize := uint64(1024 * 64)
	if initialBufSize > limit {
		initialBufSize = limit
	}

	lg.FromContext(ctx).Debug("Configuring bufio.Scanner buffer",
		"initial", datasize.ByteSize(initialBufSize),
		"limit", datasize.ByteSize(limit))

	sc.Buffer(make([]byte, int(initialBufSize)), int(limit)) //nolint:gosec // ignore overflow concern
	return sc
}
