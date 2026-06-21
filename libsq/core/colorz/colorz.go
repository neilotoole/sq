// Package colorz provides supplemental color functionality.
package colorz

import (
	"bytes"
	"io"

	"github.com/fatih/color"
	colorable "github.com/mattn/go-colorable"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// NewPrinter returns a new Printer that uses c for colorization. If c is nil,
// or if c has no effect (see [HasEffect]), the returned Printer will not
// perform colorization.
func NewPrinter(c *color.Color) Printer {
	if !HasEffect(c) {
		return monoPrinter{}
	}

	codes := ExtractSeqs(c)
	if len(codes.Prefix) == 0 {
		return monoPrinter{}
	}

	return colorPrinter{prefix: codes.Prefix, suffix: codes.Suffix}
}

// Printer provides color-aware printing.
type Printer interface {
	// Fragment prints colorized b to w. If b is empty, w is not written to.
	// Colorization breaks if b contains internal line breaks; instead use
	// [Printer.Block].
	Fragment(w io.Writer, b []byte) (n int, err error)

	// Line prints colorized b to w, always terminating with a newline. If b is
	// empty, a single newline is printed. Colorization breaks if b contains
	// internal line breaks; instead use [Printer.Block].
	Line(w io.Writer, b []byte) (n int, err error)

	// Block prints colorized b to w, preserving line breaks. If b terminates with
	// a newline, that newline is written to w; if not, a terminating newline is
	// not written. If b is empty, w is not written to.
	Block(w io.Writer, b []byte) (n int, err error)
}

var _ Printer = (*monoPrinter)(nil)

type monoPrinter struct{}

func (p monoPrinter) Block(w io.Writer, b []byte) (n int, err error) {
	n, err = w.Write(b)
	return n, errz.Err(err)
}

var newline = []byte{'\n'}

func (p monoPrinter) Line(w io.Writer, b []byte) (n int, err error) {
	if len(b) == 0 {
		n, err = w.Write(newline)
		return n, errz.Err(err)
	}

	n, err = w.Write(b)
	if err != nil {
		return n, errz.Err(err)
	}

	if b[len(b)-1] == '\n' {
		return n, nil
	}

	n2, err := w.Write(newline)
	return n + n2, errz.Err(err)
}

func (p monoPrinter) Fragment(w io.Writer, b []byte) (n int, err error) {
	n, err = w.Write(b)
	return n, errz.Err(err)
}

var _ Printer = (*colorPrinter)(nil)

type colorPrinter struct {
	prefix, suffix []byte
}

func (p colorPrinter) Fragment(w io.Writer, b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	n, err = w.Write(p.prefix)
	if err != nil {
		return n, errz.Err(err)
	}

	var n2 int
	n2, err = w.Write(b)
	n += n2
	if err != nil {
		return n, errz.Err(err)
	}

	n2, err = w.Write(p.suffix)
	n += n2
	if err != nil {
		return n, errz.Err(err)
	}

	return n, nil
}

func (p colorPrinter) Line(w io.Writer, b []byte) (n int, err error) {
	if len(b) == 0 {
		n, err = w.Write(newline)
		return n, errz.Err(err)
	}

	n, err = w.Write(p.prefix)
	if err != nil {
		return n, errz.Err(err)
	}

	var n2 int
	n2, err = w.Write(b)
	n += n2
	if err != nil {
		return n, errz.Err(err)
	}

	n2, err = w.Write(p.suffix)
	n += n2
	if err != nil {
		return n, errz.Err(err)
	}

	if b[len(b)-1] == '\n' {
		return n, nil
	}

	n2, err = w.Write(newline)
	return n + n2, errz.Err(err)
}

func (p colorPrinter) Block(w io.Writer, b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}

	// Split on '\n' manually rather than via bufio.Scanner. Scanner caps a
	// single line at bufio.MaxScanTokenSize (64KB) and strips a '\r' preceding
	// the '\n'; doing it by hand removes the size limit and preserves the exact
	// bytes (including any '\r') of each line.
	var n2 int
	for len(b) > 0 {
		line, rest, hasNewline := bytes.Cut(b, newline)
		b = rest

		// Only wrap non-empty line content in color sequences; a blank line
		// emits just its newline, with no empty colorized span.
		if len(line) > 0 {
			n2, err = w.Write(p.prefix)
			n += n2
			if err != nil {
				return n, errz.Err(err)
			}

			n2, err = w.Write(line)
			n += n2
			if err != nil {
				return n, errz.Err(err)
			}

			n2, err = w.Write(p.suffix)
			n += n2
			if err != nil {
				return n, errz.Err(err)
			}
		}

		if hasNewline {
			n2, err = w.Write(newline)
			n += n2
			if err != nil {
				return n, errz.Err(err)
			}
		}
	}

	return n, nil
}

// HasEffect returns true if c has an effect, i.e. if c is non-nil and produces
// a non-empty color sequence. For example, if [color.Color.DisableColor] is
// invoked on c, HasEffect returns false.
func HasEffect(c *color.Color) bool {
	if c == nil {
		return false
	}

	return c.Sprint(" ") != " "
}

// Seqs represents the prefix and suffix bytes for a terminal color sequence.
// Use [ExtractSeqs] to build a Seqs from a [color.Color].
//
// REVISIT: Life would be simpler if we just implemented our own Color type that
// embedded fatih/color.Color. There's a lot of messing around in this pkg.
type Seqs struct {
	Prefix []byte
	Suffix []byte
}

// Write writes p to w, prefixed and suffixed by s.Prefix and s.Suffix. If p is
// empty, Write is a no-op. If s.Prefix is empty, p is written uncolored (i.e.
// without the prefix/suffix). Write does not check for internal line breaks in
// p, which could break colorization.
// Note also that Write does not return the typical (n, err) for a Write method;
// it is intended for use with types such as [bytes.Buffer] where errors are not
// a significant concern.
func (s Seqs) Write(w io.Writer, p []byte) {
	switch {
	case len(p) == 0:
		return
	case len(s.Prefix) == 0:
		// No colorization.
		_, _ = w.Write(p)
	default:
		_, _ = w.Write(s.Prefix)
		_, _ = w.Write(p)
		_, _ = w.Write(s.Suffix)
	}
}

// Writeln is like Write, but it always writes a terminating newline. If p is
// empty, only a newline is written. If p is already newline-terminated, an
// additional newline is NOT written.
func (s Seqs) Writeln(w io.Writer, p []byte) {
	switch {
	case len(p) == 0:
		_, _ = w.Write(newline)
		return
	case len(s.Prefix) == 0:
		// No colorization.
		_, _ = w.Write(p)
	default:
		_, _ = w.Write(s.Prefix)
		_, _ = w.Write(p)
		_, _ = w.Write(s.Suffix)
	}

	if p[len(p)-1] != '\n' {
		_, _ = w.Write(newline)
	}
}

// Append appends colorized p to dest, returning the result. If p is empty, dest
// is returned unmodified. If s.Prefix is empty, p is appended uncolored (i.e.
// without the prefix/suffix).
func (s Seqs) Append(dest, p []byte) []byte {
	switch {
	case len(p) == 0:
		return dest
	case len(s.Prefix) == 0:
		// No colorization.
		return append(dest, p...)
	default:
		dest = append(dest, s.Prefix...)
		dest = append(dest, p...)
		dest = append(dest, s.Suffix...)
		return dest
	}
}

// Appendln appends colorized p to dest and then a newline, returning the
// result. If p is already newline-terminated, an additional newline is NOT
// appended, matching [Seqs.Writeln].
func (s Seqs) Appendln(dest, p []byte) []byte {
	dest = s.Append(dest, p)
	if len(p) == 0 || p[len(p)-1] != '\n' {
		dest = append(dest, '\n')
	}
	return dest
}

// ExtractSeqs extracts the prefix and suffix bytes for the terminal color
// sequence produced by c. The prefix and suffix are extracted even if c is
// disabled, e.g. via [color.Color.DisableColor]. If c is nil, or if there's no
// color sequence, the zero value is returned.
func ExtractSeqs(c *color.Color) Seqs {
	var seqs Seqs

	if c == nil {
		return seqs
	}

	// Dirty hack ahead: print a space using c, then grab the bytes printed before
	// and after the space, and those are the bytes we need for the prefix and
	// suffix. There's got to be a better way to do this.

	// Make a copy because the pkg-level color.NoColor could be false.
	c2 := *c
	c2.EnableColor()

	b := []byte(c2.Sprint(" "))
	i := bytes.IndexByte(b, ' ')
	if i <= 0 {
		// Shouldn't be possible.
		return seqs
	}
	// i > 0 here, so prefix is guaranteed non-empty.
	seqs.Prefix = b[:i]
	seqs.Suffix = b[i+1:]
	return seqs
}

// Strip returns a copy of b with all terminal color sequences removed.
func Strip(b []byte) []byte {
	if len(b) == 0 {
		return b
	}

	buf := bytes.Buffer{}
	_, _ = colorable.NewNonColorable(&buf).Write(b)
	return buf.Bytes()
}
