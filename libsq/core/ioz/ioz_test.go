package ioz_test

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestDelayReader(t *testing.T) {
	t.Parallel()
	const (
		limit = 100000
		count = 15
	)

	wg := &sync.WaitGroup{}
	wg.Add(count)
	for i := range count {
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
	const val = `In Xanadu did Kubla Khan a stately pleasure dome decree`
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

func TestRenameDir(t *testing.T) {
	dir1, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	dir2, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	// Rename dir2 into dir1.
	err = ioz.RenameDir(dir2, dir1)
	require.NoError(t, err)
}

func TestNewErrorAfterBytesReader(t *testing.T) {
	wantErr := errors.New("huzzah")
	input := ""

	rdr := ioz.NewErrorAfterBytesReader([]byte(input), wantErr)
	got, err := io.ReadAll(rdr)
	require.Equal(t, input, string(got))
	require.True(t, errors.Is(err, wantErr))
}
