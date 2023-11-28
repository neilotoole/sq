package progress_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/progress"
)

func TestNewWriter(t *testing.T) {
	const limit = 1000000

	ctx := context.Background()
	pb := progress.New(ctx, os.Stdout, time.Millisecond, progress.DefaultColors())
	ctx = progress.NewContext(ctx, pb)

	src := ioz.LimitRandReader(limit)
	src = ioz.DelayReader(src, 10*time.Millisecond, true)

	dest := io.Discard
	w := progress.NewWriter(ctx, "write test", -1, dest)

	written, err := io.Copy(w, src)
	require.NoError(t, err)
	require.Equal(t, int64(limit), written)
	pb.Wait()
}

// TestNewWriter_Closer_type tests that the returned writer
// implements io.ReadCloser, or not, depending upon the type of
// the underlying writer.
func TestNewReader_Closer_type(t *testing.T) {
	ctx := context.Background()
	pb := progress.New(ctx, os.Stdout, time.Millisecond, progress.DefaultColors())
	ctx = progress.NewContext(ctx, pb)
	defer pb.Wait()

	// bytes.Buffer doesn't implement io.Closer
	buf := &bytes.Buffer{}
	gotReader := progress.NewReader(ctx, "no closer", -1, buf)
	require.NotNil(t, gotReader)
	_, isCloser := gotReader.(io.ReadCloser)

	assert.False(t, isCloser, "expected reader NOT to be io.ReadCloser but was %T",
		gotReader)

	bufCloser := io.NopCloser(buf)
	gotReader = progress.NewReader(ctx, "closer", -1, bufCloser)
	require.NotNil(t, gotReader)
	_, isCloser = gotReader.(io.ReadCloser)

	assert.True(t, isCloser, "expected reader to be io.ReadCloser but was %T",
		gotReader)
}
