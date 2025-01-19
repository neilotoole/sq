package diffdoc_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/diffdoc"
)

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
