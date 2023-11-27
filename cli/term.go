package cli

import (
	"io"
	"os"

	isatty "github.com/mattn/go-isatty"
	"golang.org/x/term"
)

// isTerminal returns true if w is a terminal.
func isTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

// isColorTerminal returns true if w is a colorable terminal.
func isColorTerminal(w io.Writer) bool {
	if w == nil {
		return false
	}

	// TODO: Add the improvements from jsoncolor:
	// https://github.com/neilotoole/jsoncolor/pull/27
	if !isTerminal(w) {
		return false
	}

	if os.Getenv("TERM") == "dumb" {
		return false
	}

	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	if isatty.IsCygwinTerminal(f.Fd()) {
		return false
	}

	return true
}
