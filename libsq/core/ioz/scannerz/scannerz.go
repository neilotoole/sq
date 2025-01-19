// Package scannerz contains functionality for scanning data, most specifically
// for working with bufio.Scanner.
package scannerz

import (
	"bufio"
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/options"
)

var OptScanTokenBufLimit = options.NewInt(
	"tuning.scan-token-buffer-limit",
	nil,
	1000*1000, // 1MB
	"Scan token buffer limit",
	`Size in bytes of the buffer used for scanning tokens.`,
	options.TagTuning,
)

// NewScanner returns a new bufio.Scanner configured via OptScanTokenBufLimit
// set on ctx.
func NewScanner(ctx context.Context, r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*64), OptScanTokenBufLimit.Get(options.FromContext(ctx)))
	return sc
}
