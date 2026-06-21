package diffdoc_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/fatih/color"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/colorz"
	"github.com/neilotoole/sq/libsq/core/diffdoc"
)

// TestColorizer_singleCharLines exercises the single-character (length == 1)
// diff lines: a bare marker with no content. Each marker must be colorized with
// the same color the multi-character path would use. This guards against the
// former bug where the single-char branch was unreachable (gated on the
// impossible length == 0) and miscolored "+" as a deletion.
func TestColorizer_singleCharLines(t *testing.T) {
	seqs := func(c *color.Color) (prefix, suffix string) {
		s := colorz.ExtractSeqs(c)
		return string(s.Prefix), string(s.Suffix)
	}

	clrs := diffdoc.NewColors()
	delP, delS := seqs(clrs.Deletion)
	insP, insS := seqs(clrs.Insertion)
	ctxP, ctxS := seqs(clrs.Context)
	cmdP, cmdS := seqs(clrs.CmdTitle)

	testCases := []struct {
		name string
		in   string
		want string
	}{
		{name: "deletion", in: "-", want: delP + "-" + delS + "\n"},
		{name: "insertion", in: "+", want: insP + "+" + insS + "\n"},
		{name: "context", in: " ", want: ctxP + " " + ctxS + "\n"},
		{name: "command", in: "x", want: cmdP + "x" + cmdS + "\n"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := diffdoc.NewColorizer(context.Background(), diffdoc.NewColors(), bytes.NewReader([]byte(tc.in)))
			got, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))
		})
	}
}

func TestNewColorizer(t *testing.T) {
	f, err := os.Open("testdata/kubla.monochrome.patch")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	fi, err := f.Stat()
	require.NoError(t, err)

	clrs := diffdoc.NewColors()
	r := diffdoc.NewColorizer(context.Background(), clrs, f)

	got := &bytes.Buffer{}
	require.NoError(t, err)

	n, err := io.Copy(got, r)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, fi.Size())

	colorFixture, err := os.ReadFile("testdata/kubla.color.patch")
	require.NoError(t, err)
	require.Equal(t, colorFixture, got.Bytes())
}
