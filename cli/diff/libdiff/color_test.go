package libdiff_test

import (
	"bytes"
	"github.com/neilotoole/sq/cli/diff/libdiff"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"testing"
)

func TestNewColorizer(t *testing.T) {
	f, err := os.Open("testdata/kubla.patch")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })
	fi, err := f.Stat()
	require.NoError(t, err)

	pr := libdiff.NewPrinting()

	r := libdiff.NewColorizer(pr, f)
	n, err := io.Copy(os.Stdout, r)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, fi.Size())
}

func TestBuf(t *testing.T) {
	buf := &bytes.Buffer{}
	buf.WriteString("huzzah")

	s := make([]byte, 10)
	n, err := buf.Read(s)
	require.NoError(t, err)
	require.Equal(t, 6, n)
}
