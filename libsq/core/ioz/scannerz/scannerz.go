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

// ConfigureScanner configures the bufio.Scanner sc with the buffer
// size taken from OptScanTokenBufLimit. The configured scanner is returned
// for fluency.
func ConfigureScanner(ctx context.Context, sc *bufio.Scanner) *bufio.Scanner {
	sc.Buffer(make([]byte, 1024*64), OptScanTokenBufLimit.Get(options.FromContext(ctx)))
	return sc
}

// NewScanner returns a new bufio.Scanner configured via ConfigureScanner.
func NewScanner(ctx context.Context, r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	ConfigureScanner(ctx, sc)
	return sc
}
