// Package termz contains a handful of terminal utilities.
package termz

import (
	"io"
	"os"

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
