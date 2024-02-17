// Package colorz provides supplemental color functionality.
package colorz

import (
	"bufio"
	"bytes"
	"io"

	"github.com/mattn/go-colorable"

	"github.com/fatih/color"

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
	return w.Write(b)
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

	sc := bufio.NewScanner(bytes.NewReader(b))
	var n2 int
	for i := 0; sc.Scan(); i++ {
		if i > 0 {
			n2, err = w.Write(newline)
			n += n2
			if err != nil {
				return n, errz.Err(err)
			}
		}
		n2, err = w.Write(p.prefix)
		n += n2
		if err != nil {
			return n, errz.Err(err)
		}

		n2, err = w.Write(sc.Bytes())
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

	if sc.Err() != nil {
		return n, errz.Err(sc.Err())
	}

	if b[len(b)-1] != '\n' {
		return n, nil
	}

	n2, err = w.Write(newline)
	return n + n2, errz.Err(err)
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
type Seqs struct {
	Prefix []byte
	Suffix []byte
}

// Write writes p to w, prefixed and suffixed by c.Prefix and c.Suffix. If c
// is the zero value, or w is nil, or p is empty, Write is no-op. Write does
// not check for internal line breaks in p, which could break colorization.
// Note also that Write does not return the typical (n, err) for a Write method;
// it is intended for use with types such as [bytes.Buffer] where errors are not
// a significant concern.
func (s Seqs) Write(w io.Writer, p []byte) {
	if len(p) == 0 || len(s.Prefix) == 0 {
		return
	}

	_, _ = w.Write(s.Prefix)
	_, _ = w.Write(p)
	_, _ = w.Write(s.Suffix)
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

var _ ByteWriter = (*bytes.Buffer)(nil)

// ByteWriter is implemented by bytes.Buffer. It's used by [Seqs.WriteByte] and
// [Seqs.WritelnByte] to avoid unnecessary allocations.
type ByteWriter interface {
	io.Writer
	WriteByte(byte) error
}

// WriteByte writes a colorized byte to w. This method is basically an
// optimization for when w is [bytes.Buffer].
func (s Seqs) WriteByte(w ByteWriter, b byte) {
	if len(s.Prefix) == 0 {
		_ = w.WriteByte(b)
		return
	}

	_, _ = w.Write(s.Prefix)
	_ = w.WriteByte(b)
	_, _ = w.Write(s.Suffix)
}

// WritelnByte writes a colorized byte and a newline to w. This method is
// basically an optimization for when w is [bytes.Buffer].
func (s Seqs) WritelnByte(w ByteWriter, b byte) {
	if len(s.Prefix) == 0 {
		_ = w.WriteByte(b)
		_, _ = w.Write(newline)
		return
	}

	_, _ = w.Write(s.Prefix)
	_ = w.WriteByte(b)
	_, _ = w.Write(s.Suffix)
	_, _ = w.Write(newline)
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
	prefix := b[:i]
	suffix := b[i+1:]

	if len(prefix) == 0 {
		return seqs
	}

	seqs.Prefix = prefix
	seqs.Suffix = suffix
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
