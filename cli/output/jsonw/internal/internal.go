package internal

import (
	"bytes"
	"strconv"

	fcolor "github.com/fatih/color"

	"github.com/neilotoole/sq/cli/output"
)

// Colors encapsulates colorization of JSON output.
type Colors struct {
	Null   Color
	Bool   Color
	Number Color
	String Color
	Key    Color
	Bytes  Color
	Time   Color
	Punc   Color
}

// AppendNull appends a colorized "null" to b.
func (c Colors) AppendNull(b []byte) []byte {
	b = append(b, c.Null.Prefix...)
	b = append(b, "null"...)
	return append(b, c.Null.Suffix...)
}

// AppendBool appends the colorized bool v to b.
func (c Colors) AppendBool(b []byte, v bool) []byte {
	b = append(b, c.Bool.Prefix...)

	if v {
		b = append(b, "true"...)
	} else {
		b = append(b, "false"...)
	}

	return append(b, c.Bool.Suffix...)
}

// AppendKey appends the colorized key v to b.
func (c Colors) AppendKey(b, v []byte) []byte {
	b = append(b, c.Key.Prefix...)
	b = append(b, v...)
	return append(b, c.Key.Suffix...)
}

// AppendInt64 appends the colorized int64 v to b.
func (c Colors) AppendInt64(b []byte, v int64) []byte {
	b = append(b, c.Number.Prefix...)
	b = strconv.AppendInt(b, v, 10)
	return append(b, c.Number.Suffix...)
}

// AppendUint64 appends the colorized uint64 v to b.
func (c Colors) AppendUint64(b []byte, v uint64) []byte {
	b = append(b, c.Number.Prefix...)
	b = strconv.AppendUint(b, v, 10)
	return append(b, c.Number.Suffix...)
}

// AppendPunc appends the colorized punctuation mark v to b.
func (c Colors) AppendPunc(b []byte, v byte) []byte {
	b = append(b, c.Punc.Prefix...)
	b = append(b, v)
	return append(b, c.Punc.Suffix...)
}

// Color is used to render terminal colors. The Prefix
// value is written, then the actual value, then the suffix.
type Color struct {
	// Prefix is the terminal color code prefix to print before the value (may be empty).
	Prefix []byte

	// Suffix is the terminal color code suffix to print after the value (may be empty).
	Suffix []byte
}

// newColor creates a Color instance from a fatih/color instance.
func newColor(c *fcolor.Color) Color {
	// Dirty conversion function ahead: print
	// a space using c, then grab the bytes printed
	// before and after the space, and those are the
	// bytes we need for the prefix and suffix.

	if c == nil {
		return Color{}
	}

	// Make a copy because the pkg-level color.NoColor could be false.
	c2 := *c
	c2.EnableColor()

	b := []byte(c2.Sprint(" "))
	i := bytes.IndexByte(b, ' ')
	if i <= 0 {
		return Color{}
	}

	return Color{Prefix: b[:i], Suffix: b[i+1:]}
}

// NewColors builds a Colors instance from a Formatting instance.
func NewColors(fm *output.Formatting) Colors {
	if fm == nil || fm.IsMonochrome() {
		return Colors{}
	}

	return Colors{
		Null:   newColor(fm.Null),
		Bool:   newColor(fm.Bool),
		Bytes:  newColor(fm.Bytes),
		Number: newColor(fm.Number),
		String: newColor(fm.String),
		Time:   newColor(fm.Datetime),
		Key:    newColor(fm.Key),
		Punc:   newColor(fm.Punc),
	}
}
