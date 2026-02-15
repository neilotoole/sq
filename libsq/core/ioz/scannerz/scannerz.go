// Package scannerz contains functionality for scanning data, most specifically
// for working with bufio.Scanner. See scannerz.NewScanner.
package scannerz

import (
	"bufio"
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/datasize"
	"github.com/neilotoole/sq/libsq/core/options"
)

// OptScanBufLimit is an Opt for configuring the buffer size of a bufio.Scanner
// returned from scannerz.NewScanner.
var OptScanBufLimit = datasize.NewOpt(
	"tuning.scan-buffer-limit",
	nil,
	datasize.MustParseString("32MB"),
	"Scan token buffer limit",
	`Maximum size of the buffer used for scanning tokens. The buffer will start
small and grow as needed, but will not exceed this limit.

Use units B, KB, MB, GB, etc. For example, 64KB, or 10MB. If no unit specified,
bytes are assumed.`,
	options.TagTuning,
)

// NewScanner returns a new bufio.Scanner configured via OptScanBufLimit
// set on ctx.
func NewScanner(ctx context.Context, r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)

	limit := OptScanBufLimit.Get(options.FromContext(ctx)).Bytes()
	initial := min(
		// 4096 is the default initial bufio.Scanner buffer size.
		uint64(4096), limit)

	sc.Buffer(make([]byte, int(initial)), int(limit)) //nolint:gosec // ignore overflow concern
	return sc
}
