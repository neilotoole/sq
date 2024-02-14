package colorz

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestPrinter(t *testing.T) {
	// TODO: write actual tests. Currently the test is just
	// executed manually and the output is visually inspected.
	// Not great, Bob.

	previous := color.NoColor
	t.Cleanup(func() {
		color.NoColor = previous
	})

	color.NoColor = false

	c := color.New(color.FgBlue)
	p := NewPrinter(c)

	p.Fragment(os.Stdout, []byte("hello"))
	p.Fragment(os.Stdout, []byte("world"))
	fmt.Fprintln(os.Stdout)

	p.Line(os.Stdout, []byte("single line"))

	p.Block(os.Stdout, []byte("Hello,\nworld!"))
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
