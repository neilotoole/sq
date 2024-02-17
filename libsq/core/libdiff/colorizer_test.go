package libdiff_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/libdiff"
)

func TestNewColorizer(t *testing.T) {
	f, err := os.Open("testdata/kubla.monochrome.patch")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	fi, err := f.Stat()
	require.NoError(t, err)

	clrs := libdiff.NewColors()
	r := libdiff.NewColorizer(clrs, f)

	got := &bytes.Buffer{}
	require.NoError(t, err)

	n, err := io.Copy(got, r)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, fi.Size())

	colorFixture, err := os.ReadFile("testdata/kubla.color.patch")
	require.NoError(t, err)
	require.Equal(t, colorFixture, got.Bytes())
}
