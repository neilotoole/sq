package termz

import (
	"io"
	"os"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

// IsTerminal returns true if w is a terminal.
func IsTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

// IsColorTerminal returns true if w is a colorable terminal.
// It respects [NO_COLOR], [FORCE_COLOR] and TERM=dumb environment variables.
//
// Acknowledgement: This function is lifted from neilotoole/jsoncolor, but
// it was contributed by @hermannm.
// - https://github.com/neilotoole/jsoncolor/pull/27
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
	fd := f.Fd()

	console := windows.Handle(fd)
	var mode uint32
	if err := windows.GetConsoleMode(console, &mode); err != nil {
		return false
	}

	var want uint32 = windows.ENABLE_PROCESSED_OUTPUT | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if (mode & want) == want {
		return true
	}

	mode |= want
	if err := windows.SetConsoleMode(console, mode); err != nil {
		return false
	}

	return true
}
