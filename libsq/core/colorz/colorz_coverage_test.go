package colorz

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/errz"
)

// failWriter is an io.Writer (and ByteWriter) that accepts up to failAfter
// bytes across all writes, then returns an error. It performs partial writes,
// returning the count actually accepted alongside the error, mimicking a real
// short-write failure. It's used to exercise the error-handling branches of
// the colorz writers.
type failWriter struct {
	failAfter int
	written   int
}

var errBoom = errz.New("failWriter: boom")

func (w *failWriter) Write(p []byte) (int, error) {
	remaining := w.failAfter - w.written
	if remaining <= 0 {
		return 0, errBoom
	}
	if len(p) > remaining {
		w.written += remaining
		return remaining, errBoom
	}
	w.written += len(p)
	return len(p), nil
}

func (w *failWriter) WriteByte(_ byte) error {
	if w.failAfter-w.written <= 0 {
		return errBoom
	}
	w.written++
	return nil
}

// newColorPrinter returns a colorPrinter with deterministic, easily-readable
// prefix/suffix sequences, plus the byte lengths of each. Using synthetic
// sequences keeps the byte-accounting assertions independent of fatih/color's
// exact escape codes.
func newColorPrinter() (p colorPrinter, prefixLen, suffixLen int) {
	prefix := []byte("<P>")
	suffix := []byte("<S>")
	return colorPrinter{prefix: prefix, suffix: suffix}, len(prefix), len(suffix)
}

func TestMonoPrinter_Fragment(t *testing.T) {
	var p monoPrinter

	buf := &bytes.Buffer{}
	n, err := p.Fragment(buf, []byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "hello", buf.String())

	// Empty input: the interface contract says w is not written to. mono just
	// issues a zero-length write, which writes nothing observable.
	buf.Reset()
	n, err = p.Fragment(buf, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.Empty(t, buf.String())

	// Write error is surfaced.
	n, err = p.Fragment(&failWriter{failAfter: 0}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, 0, n)
}

func TestMonoPrinter_Line(t *testing.T) {
	var p monoPrinter

	// Empty input writes a single newline.
	buf := &bytes.Buffer{}
	n, err := p.Line(buf, nil)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "\n", buf.String())

	// Non-terminated input gets a trailing newline.
	buf.Reset()
	n, err = p.Line(buf, []byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 6, n)
	require.Equal(t, "hello\n", buf.String())

	// Already-terminated input is not double-terminated.
	buf.Reset()
	n, err = p.Line(buf, []byte("hello\n"))
	require.NoError(t, err)
	require.Equal(t, 6, n)
	require.Equal(t, "hello\n", buf.String())

	// Error writing the lone newline (empty input).
	n, err = p.Line(&failWriter{failAfter: 0}, nil)
	require.Error(t, err)
	require.Equal(t, 0, n)

	// Error writing the body.
	n, err = p.Line(&failWriter{failAfter: 0}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, 0, n)

	// Error writing the trailing newline (body succeeds).
	n, err = p.Line(&failWriter{failAfter: 5}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, 5, n)
}

func TestMonoPrinter_Block(t *testing.T) {
	var p monoPrinter

	buf := &bytes.Buffer{}
	n, err := p.Block(buf, []byte("Hello,\nworld!"))
	require.NoError(t, err)
	require.Equal(t, 13, n)
	require.Equal(t, "Hello,\nworld!", buf.String())

	// Error path.
	n, err = p.Block(&failWriter{failAfter: 0}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, 0, n)
}

func TestColorPrinter_Fragment(t *testing.T) {
	p, pl, sl := newColorPrinter()

	buf := &bytes.Buffer{}
	n, err := p.Fragment(buf, []byte("hello"))
	require.NoError(t, err)
	require.Equal(t, pl+5+sl, n)
	require.Equal(t, "<P>hello<S>", buf.String())

	// Empty input is a no-op.
	buf.Reset()
	n, err = p.Fragment(buf, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.Empty(t, buf.String())

	// Error writing prefix.
	n, err = p.Fragment(&failWriter{failAfter: 0}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, 0, n)

	// Error writing body (prefix succeeds).
	n, err = p.Fragment(&failWriter{failAfter: pl}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, pl, n)

	// Error writing suffix (prefix + body succeed).
	n, err = p.Fragment(&failWriter{failAfter: pl + 5}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, pl+5, n)
}

func TestColorPrinter_Line(t *testing.T) {
	p, pl, sl := newColorPrinter()

	// Empty input writes a single (uncolored) newline.
	buf := &bytes.Buffer{}
	n, err := p.Line(buf, nil)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, "\n", buf.String())

	// Non-terminated input.
	buf.Reset()
	n, err = p.Line(buf, []byte("hello"))
	require.NoError(t, err)
	require.Equal(t, pl+5+sl+1, n)
	require.Equal(t, "<P>hello<S>\n", buf.String())

	// Already-terminated input: the trailing newline is inside the colorized
	// span and no extra newline is appended.
	buf.Reset()
	n, err = p.Line(buf, []byte("hello\n"))
	require.NoError(t, err)
	require.Equal(t, pl+6+sl, n)
	require.Equal(t, "<P>hello\n<S>", buf.String())

	// Error writing the lone newline (empty input).
	n, err = p.Line(&failWriter{failAfter: 0}, nil)
	require.Error(t, err)
	require.Equal(t, 0, n)

	// Error writing prefix.
	n, err = p.Line(&failWriter{failAfter: 0}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, 0, n)

	// Error writing body.
	n, err = p.Line(&failWriter{failAfter: pl}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, pl, n)

	// Error writing suffix.
	n, err = p.Line(&failWriter{failAfter: pl + 5}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, pl+5, n)

	// Error writing the trailing newline.
	n, err = p.Line(&failWriter{failAfter: pl + 5 + sl}, []byte("hello"))
	require.Error(t, err)
	require.Equal(t, pl+5+sl, n)
}

func TestColorPrinter_Block(t *testing.T) {
	p, pl, sl := newColorPrinter()

	// Multi-line, not newline-terminated.
	buf := &bytes.Buffer{}
	n, err := p.Block(buf, []byte("Hello,\nworld!"))
	require.NoError(t, err)
	require.Equal(t, "<P>Hello,<S>\n<P>world!<S>", buf.String())
	require.Equal(t, pl+6+sl+1+pl+6+sl, n)

	// Newline-terminated input retains the trailing newline.
	buf.Reset()
	n, err = p.Block(buf, []byte("Hello,\nworld!\n"))
	require.NoError(t, err)
	require.Equal(t, "<P>Hello,<S>\n<P>world!<S>\n", buf.String())
	require.Equal(t, pl+6+sl+1+pl+6+sl+1, n)

	// Single line, newline-terminated.
	buf.Reset()
	n, err = p.Block(buf, []byte("solo\n"))
	require.NoError(t, err)
	require.Equal(t, "<P>solo<S>\n", buf.String())
	require.Equal(t, pl+4+sl+1, n)

	// Empty input is a no-op.
	buf.Reset()
	n, err = p.Block(buf, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	require.Empty(t, buf.String())
}

func TestColorPrinter_Block_errors(t *testing.T) {
	p, pl, sl := newColorPrinter()
	const body = "ab\ncd" // two lines: "ab", "cd"

	// Error writing prefix on the first line.
	n, err := p.Block(&failWriter{failAfter: 0}, []byte(body))
	require.Error(t, err)
	require.Equal(t, 0, n)

	// Error writing the body of the first line.
	n, err = p.Block(&failWriter{failAfter: pl}, []byte(body))
	require.Error(t, err)
	require.Equal(t, pl, n)

	// Error writing the suffix of the first line.
	n, err = p.Block(&failWriter{failAfter: pl + 2}, []byte(body))
	require.Error(t, err)
	require.Equal(t, pl+2, n)

	// Error writing the inter-line newline (before the second line).
	n, err = p.Block(&failWriter{failAfter: pl + 2 + sl}, []byte(body))
	require.Error(t, err)
	require.Equal(t, pl+2+sl, n)

	// Error writing the trailing newline.
	firstLine := pl + 4 + sl // "<P>solo<S>"
	n, err = p.Block(&failWriter{failAfter: firstLine}, []byte("solo\n"))
	require.Error(t, err)
	require.Equal(t, firstLine, n)
}

// TestColorPrinter_Block_longLine verifies that Block handles a line longer
// than bufio.MaxScanTokenSize (64KB), which the previous bufio.Scanner-based
// implementation could not.
func TestColorPrinter_Block_longLine(t *testing.T) {
	p, pl, sl := newColorPrinter()
	// A single line longer than the old 64*1024 scanner token limit.
	line := bytes.Repeat([]byte("x"), 70*1024)

	buf := &bytes.Buffer{}
	n, err := p.Block(buf, line)
	require.NoError(t, err)
	require.Equal(t, pl+len(line)+sl, n)
	require.Equal(t, "<P>"+string(line)+"<S>", buf.String())
}

// TestColorPrinter_Block_preservesCarriageReturn verifies that Block preserves
// a '\r' preceding a '\n'; the bytes of each line are emitted verbatim.
func TestColorPrinter_Block_preservesCarriageReturn(t *testing.T) {
	p, _, _ := newColorPrinter()

	buf := &bytes.Buffer{}
	_, err := p.Block(buf, []byte("a\r\nb"))
	require.NoError(t, err)
	// The '\r' is retained inside the first colorized span.
	require.Equal(t, "<P>a\r<S>\n<P>b<S>", buf.String())
}

func TestNewPrinter_mono(t *testing.T) {
	// Nil color yields a monoPrinter.
	_, ok := NewPrinter(nil).(monoPrinter)
	require.True(t, ok, "nil color should yield monoPrinter")

	// A color with no effect yields a monoPrinter.
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	c := color.New(color.FgBlue)
	c.DisableColor()
	_, ok = NewPrinter(c).(monoPrinter)
	require.True(t, ok, "disabled color should yield monoPrinter")
}

func TestNewPrinter_color(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")

	previous := color.NoColor
	t.Cleanup(func() { color.NoColor = previous })
	color.NoColor = false

	c := color.New(color.FgBlue)
	_, ok := NewPrinter(c).(colorPrinter)
	require.True(t, ok, "effective color should yield colorPrinter")
}

func TestExtractSeqs(t *testing.T) {
	// Nil color returns the zero value.
	require.Equal(t, Seqs{}, ExtractSeqs(nil))

	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")

	// Extraction works even when the color is disabled, because ExtractSeqs
	// enables a copy.
	c := color.New(color.FgBlue)
	c.DisableColor()
	seqs := ExtractSeqs(c)
	require.NotEmpty(t, seqs.Prefix)
	require.NotEmpty(t, seqs.Suffix)
	require.Equal(t, "\x1b[34m", string(seqs.Prefix))
	require.Equal(t, "\x1b[0m", string(seqs.Suffix))
}

func TestSeqs_Write(t *testing.T) {
	s := Seqs{Prefix: []byte("<P>"), Suffix: []byte("<S>")}

	buf := &bytes.Buffer{}
	s.Write(buf, []byte("hi"))
	require.Equal(t, "<P>hi<S>", buf.String())

	// Empty payload is a no-op.
	buf.Reset()
	s.Write(buf, nil)
	require.Empty(t, buf.String())

	// Empty prefix (zero-ish Seqs): p is written uncolored, not dropped.
	buf.Reset()
	(Seqs{}).Write(buf, []byte("hi"))
	require.Equal(t, "hi", buf.String())
}

func TestSeqs_Writeln(t *testing.T) {
	s := Seqs{Prefix: []byte("<P>"), Suffix: []byte("<S>")}

	// Empty payload writes only a newline.
	buf := &bytes.Buffer{}
	s.Writeln(buf, nil)
	require.Equal(t, "\n", buf.String())

	// Colorized, not newline-terminated.
	buf.Reset()
	s.Writeln(buf, []byte("hi"))
	require.Equal(t, "<P>hi<S>\n", buf.String())

	// Colorized, already newline-terminated: no extra newline.
	buf.Reset()
	s.Writeln(buf, []byte("hi\n"))
	require.Equal(t, "<P>hi\n<S>", buf.String())

	// No prefix: payload written uncolored, with a trailing newline.
	buf.Reset()
	(Seqs{}).Writeln(buf, []byte("hi"))
	require.Equal(t, "hi\n", buf.String())

	// No prefix, already newline-terminated.
	buf.Reset()
	(Seqs{}).Writeln(buf, []byte("hi\n"))
	require.Equal(t, "hi\n", buf.String())
}

func TestSeqs_Append(t *testing.T) {
	s := Seqs{Prefix: []byte("<P>"), Suffix: []byte("<S>")}

	got := s.Append([]byte("pre:"), []byte("hi"))
	require.Equal(t, "pre:<P>hi<S>", string(got))

	// Empty payload returns dest unmodified.
	got = s.Append([]byte("pre:"), nil)
	require.Equal(t, "pre:", string(got))

	// Empty prefix: p is appended uncolored (consistent with Writeln/PutByte).
	got = (Seqs{}).Append([]byte("pre:"), []byte("hi"))
	require.Equal(t, "pre:hi", string(got))
}

func TestSeqs_Appendln(t *testing.T) {
	s := Seqs{Prefix: []byte("<P>"), Suffix: []byte("<S>")}

	got := s.Appendln([]byte("pre:"), []byte("hi"))
	require.Equal(t, "pre:<P>hi<S>\n", string(got))

	// Already newline-terminated: no extra newline (matches Writeln).
	got = s.Appendln([]byte("pre:"), []byte("hi\n"))
	require.Equal(t, "pre:<P>hi\n<S>", string(got))

	// Empty payload: just the newline is appended.
	got = s.Appendln([]byte("pre:"), nil)
	require.Equal(t, "pre:\n", string(got))

	// Empty prefix: p is appended uncolored, then the trailing newline.
	got = (Seqs{}).Appendln([]byte("pre:"), []byte("hi"))
	require.Equal(t, "pre:hi\n", string(got))
}

func TestSeqs_PutByte(t *testing.T) {
	s := Seqs{Prefix: []byte("<P>"), Suffix: []byte("<S>")}

	buf := &bytes.Buffer{}
	s.PutByte(buf, 'x')
	require.Equal(t, "<P>x<S>", buf.String())

	// No prefix: the byte is written uncolored.
	buf.Reset()
	(Seqs{}).PutByte(buf, 'x')
	require.Equal(t, "x", buf.String())
}

func TestSeqs_PutlnByte(t *testing.T) {
	s := Seqs{Prefix: []byte("<P>"), Suffix: []byte("<S>")}

	buf := &bytes.Buffer{}
	s.PutlnByte(buf, 'x')
	require.Equal(t, "<P>x<S>\n", buf.String())

	// No prefix: byte plus newline, uncolored.
	buf.Reset()
	(Seqs{}).PutlnByte(buf, 'x')
	require.Equal(t, "x\n", buf.String())
}

func TestStrip(t *testing.T) {
	// Empty input returns empty.
	require.Empty(t, Strip(nil))
	require.Empty(t, Strip([]byte{}))

	// Color sequences are removed; plain text is preserved.
	colored := []byte("\x1b[34mhello\x1b[0m world")
	require.Equal(t, "hello world", string(Strip(colored)))

	// Text with no sequences is unchanged.
	require.Equal(t, "plain", string(Strip([]byte("plain"))))

	// Multi-line colored content.
	in := []byte("\x1b[34ma\x1b[0m\n\x1b[31mb\x1b[0m")
	require.Equal(t, "a\nb", string(Strip(in)))
}

// TestStrip_roundTrip confirms that stripping the output of a colorPrinter
// recovers the original text (for single-line fragments).
func TestStrip_roundTrip(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")

	previous := color.NoColor
	t.Cleanup(func() { color.NoColor = previous })
	color.NoColor = false

	p := NewPrinter(color.New(color.FgRed, color.Bold))
	buf := &bytes.Buffer{}
	_, err := p.Fragment(buf, []byte("round trip"))
	require.NoError(t, err)
	require.NotEqual(t, "round trip", buf.String(), "expected colorization")
	require.Equal(t, "round trip", string(Strip(buf.Bytes())))
}

// TestColorPrinter_Block_blankLine verifies that a blank interior line emits
// only its newline, with no empty colorized span.
func TestColorPrinter_Block_blankLine(t *testing.T) {
	p, _, _ := newColorPrinter()
	buf := &bytes.Buffer{}
	_, err := p.Block(buf, []byte("a\n\nb"))
	require.NoError(t, err)
	require.Equal(t, "<P>a<S>\n\n<P>b<S>", buf.String())
	require.False(t, strings.Contains(buf.String(), "<P><S>"),
		"blank line should not be wrapped in an empty color span")
}
