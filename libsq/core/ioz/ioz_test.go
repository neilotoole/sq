package ioz_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/checksum"
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

func TestWriteToFile(t *testing.T) {
	const val = `In Zanadu did Kubla Khan a stately pleasure dome decree`
	ctx := context.Background()
	dir := t.TempDir()

	fp := filepath.Join(dir, "not_existing_intervening_dir", "test.txt")
	written, err := ioz.WriteToFile(ctx, fp, strings.NewReader(val))
	require.NoError(t, err)
	require.Equal(t, int64(len(val)), written)

	got, err := os.ReadFile(fp)
	require.NoError(t, err)
	require.Equal(t, val, string(got))
}
