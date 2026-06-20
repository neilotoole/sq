package footer

import (
	"io"
	"os"
	"regexp"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

var ansiRe = regexp.MustCompile("\033\\[(?:[0-9]{1,3}(?:;[0-9]{1,3})*)?[mK]")

// terminalWidth returns stderr/stdout terminal width in columns, or 0 if unknown.
func terminalWidth(w interface{ Fd() uintptr }) int {
	width, _, err := term.GetSize(int(w.Fd()))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func terminalWidthFromWriter(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	return terminalWidth(f)
}

// displayWidth returns the visible width of s, ignoring ANSI escape sequences.
func displayWidth(s string) int {
	return runewidth.StringWidth(ansiRe.ReplaceAllString(s, ""))
}

// alignRight pads s with leading spaces so it ends at column width.
// If width is 0 or s is wider than width, s is returned unchanged.
func alignRight(s string, width int) string {
	if width <= 0 {
		return s
	}
	gap := width - displayWidth(s)
	if gap <= 0 {
		return s
	}
	return strings.Repeat(" ", gap) + s
}
