package ioz_test

import (
	"bytes"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
)

func TestMarshalYAML(t *testing.T) {
	m := map[string]any{
		"hello": `sqlserver://sakila:p_ss"**W0rd@222.75.174.219?database=sakila`,
	}

	b, err := ioz.MarshalYAML(m)
	require.NoError(t, err)
	require.NotNil(t, b)
}

func TestChecksums(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "sq-test-*")
	require.NoError(t, err)
	_, err = io.WriteString(f, "huzzah")
	require.NoError(t, err)
	assert.NoError(t, f.Close())

	buf := &bytes.Buffer{}

	gotSum1, err := ioz.FileChecksum(f.Name())
	require.NoError(t, err)
	t.Logf("gotSum1: %s  %s", gotSum1, f.Name())
	require.NoError(t, ioz.WriteChecksum(buf, gotSum1, f.Name()))

	gotSums, err := ioz.ReadChecksums(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	require.Len(t, gotSums, 1)
	require.Equal(t, gotSum1, gotSums[f.Name()])

	// Make some changes to the file and verify that the checksums differ.
	f, err = os.OpenFile(f.Name(), os.O_APPEND|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	_, err = io.WriteString(f, "more huzzah")
	require.NoError(t, err)
	assert.NoError(t, f.Close())
	gotSum2, err := ioz.FileChecksum(f.Name())
	require.NoError(t, err)
	t.Logf("gotSum2: %s  %s", gotSum2, f.Name())
	require.NoError(t, ioz.WriteChecksum(buf, gotSum1, f.Name()))
	require.NotEqual(t, gotSum1, gotSum2)
}

func TestDelayReader(t *testing.T) {
	t.Parallel()
	const (
		limit = 100000
		count = 15
	)

	wg := &sync.WaitGroup{}
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			randRdr := ioz.LimitRandReader(limit)
			r := ioz.DelayReader(randRdr, 150*time.Millisecond, true)
			start := time.Now()
			_, err := io.ReadAll(r)
			elapsed := time.Since(start)
			t.Logf("%2d: Elapsed: %s", i, elapsed)
			require.NoError(t, err)
		}(i)
	}

	wg.Wait()
}
