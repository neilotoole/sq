package checksum_test

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
)

func TestSum(t *testing.T) {
	got := checksum.Sum(nil)
	require.Equal(t, "", got)
	got = checksum.Sum([]byte{})
	require.Equal(t, "", got)
	got = checksum.Sum([]byte("hello world"))
	assert.Equal(t, "d4a1185", got)
}

func TestChecksums(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "sq-test-*")
	require.NoError(t, err)
	_, err = io.WriteString(f, "huzzah")
	require.NoError(t, err)
	assert.NoError(t, f.Close())

	buf := &bytes.Buffer{}

	gotSum1, err := checksum.ForFile(f.Name())
	require.NoError(t, err)
	t.Logf("gotSum1: %s  %s", gotSum1, f.Name())
	require.NoError(t, checksum.Write(buf, gotSum1, f.Name()))

	gotSums, err := checksum.Read(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	require.Len(t, gotSums, 1)
	require.Equal(t, gotSum1, gotSums[f.Name()])

	// Make some changes to the file and verify that the checksums differ.
	f, err = os.OpenFile(f.Name(), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = io.WriteString(f, "more huzzah")
	require.NoError(t, err)
	assert.NoError(t, f.Close())
	gotSum2, err := checksum.ForFile(f.Name())
	require.NoError(t, err)
	t.Logf("gotSum2: %s  %s", gotSum2, f.Name())
	require.NoError(t, checksum.Write(buf, gotSum1, f.Name()))
	require.NotEqual(t, gotSum1, gotSum2)
}
