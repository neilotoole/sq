//go:build !windows

package termz

import (
	"io"
	"os"

	"golang.org/x/term"
)

// IsColorTerminal returns true if w is a colorable terminal.
// It respects [NO_COLOR], [FORCE_COLOR] and TERM=dumb environment variables.
//
// [NO_COLOR]: https://no-color.org/
// [FORCE_COLOR]: https://force-color.org/
func IsColorTerminal(w io.Writer) bool {
	if enabled, ok := colorEnvOverride(); ok {
		return enabled
	}

	if w == nil {
		return false
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	return term.IsTerminal(int(f.Fd()))
}
