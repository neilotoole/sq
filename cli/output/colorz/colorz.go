// Package colorz provides supplemental color functionality.
package colorz

import (
	"bufio"
	"bytes"
	"github.com/fatih/color"
	"github.com/neilotoole/sq/libsq/core/errz"
	"io"
)

// NewPrinter returns a new Printer that uses c for colorization. If c is nil,
// or if c has no effect (see [HasEffect]), the returned Printer will not
// perform colorization.
func NewPrinter(c *color.Color) Printer {
	if !HasEffect(c) {
		return monoPrinter{}
	}

	prefix, suffix := ExtractCodes(c)
	if len(prefix) == 0 {
		return monoPrinter{}
	}

	return colorPrinter{prefix: prefix, suffix: suffix}
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

// ExtractCodes extracts the prefix and suffix bytes for the terminal color
// sequence produced by c. The prefix and suffix are extracted even if c is
// disabled, e.g. via [color.Color.DisableColor]. If c is nil, or if there's no
// color sequence, the returned values will be nil.
func ExtractCodes(c *color.Color) (prefix, suffix []byte) {
	if c == nil {
		return nil, nil
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
		return nil, nil
	}
	prefix = b[:i]
	suffix = b[i+1:]

	if len(prefix) == 0 {
		return nil, nil
	}

	return prefix, suffix
}
