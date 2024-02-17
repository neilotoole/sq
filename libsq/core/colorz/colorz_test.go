package colorz

import (
	"bytes"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
)

func TestPrinter(t *testing.T) {
	previous := color.NoColor
	t.Cleanup(func() {
		color.NoColor = previous
	})

	color.NoColor = false

	var (
		c   = color.New(color.FgBlue)
		p   = NewPrinter(c)
		buf = &bytes.Buffer{}
		n   int
		err error
	)

	n, err = p.Fragment(buf, []byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 14, n)
	require.Equal(t, "\x1b[34mhello\x1b[0m", buf.String())

	n, err = p.Fragment(buf, []byte("world"))
	require.NoError(t, err)
	require.Equal(t, 14, n)
	require.Equal(t, "\x1b[34mhello\x1b[0m\x1b[34mworld\x1b[0m", buf.String())

	buf.Reset()
	n, err = p.Line(buf, []byte("single line"))
	require.NoError(t, err)
	require.Equal(t, 21, n)
	require.Equal(t, "\x1b[34msingle line\x1b[0m\n", buf.String())

	buf.Reset()
	n, err = p.Block(buf, []byte("Hello,\nworld!"))
	require.NoError(t, err)
	require.Equal(t, 31, n)
	require.Equal(t, "\x1b[34mHello,\x1b[0m\n\x1b[34mworld!\x1b[0m", buf.String())
}

func TestHasEffect(t *testing.T) {
	c := color.New(color.FgBlue)
	c.EnableColor()
	got := HasEffect(c)
	require.True(t, got)

	c.DisableColor()
	got = HasEffect(c)
	require.False(t, got)

	c.EnableColor()
	got = HasEffect(c)
	require.True(t, got)

	got = HasEffect(nil)
	require.False(t, got)

	previous := color.NoColor
	t.Cleanup(func() {
		color.NoColor = previous
	})

	color.NoColor = true
	c = color.New(color.FgBlue)
	got = HasEffect(c)
	require.False(t, got)

	color.NoColor = false
	c = color.New(color.FgBlue)
	got = HasEffect(c)
	require.True(t, got)
}
