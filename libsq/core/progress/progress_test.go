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
	t.Parallel()

	const limit = 1000000

	ctx := context.Background()
	pb := progress.New(ctx, io.Discard, time.Millisecond, progress.DefaultColors())
	ctx = progress.NewContext(ctx, pb)

	src := ioz.LimitRandReader(limit)
	src = ioz.DelayReader(src, 10*time.Millisecond, true)

	dest := io.Discard
	w := progress.NewWriter(ctx, "write test", -1, dest)

	written, err := io.Copy(w, src)
	require.NoError(t, err)
	require.Equal(t, int64(limit), written)
	pb.Stop()
}

// TestNewWriter_Closer tests that the returned writer
// implements io.WriteCloser regardless of whether the
// underlying writer does.
func TestNewWriter_Closer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pb := progress.New(ctx, os.Stdout, time.Millisecond, progress.DefaultColors())
	ctx = progress.NewContext(ctx, pb)
	defer pb.Stop()

	// bytes.Buffer doesn't implement io.Closer
	buf := &bytes.Buffer{}
	gotWriter := progress.NewWriter(ctx, "no closer", -1, buf)
	require.NotNil(t, gotWriter)
	_, isCloser := gotWriter.(io.WriteCloser)
	assert.True(t, isCloser, "expected writer to be io.WriteCloser, but was %T",
		gotWriter)

	bufCloser := ioz.WriteCloser(buf)
	gotWriter = progress.NewWriter(ctx, "no closer", -1, bufCloser)
	require.NotNil(t, gotWriter)
	_, isCloser = gotWriter.(io.WriteCloser)
	assert.True(t, isCloser, "expected writer to implement io.WriteCloser, but was %T",
		gotWriter)
}

// TestNewReader_Closer tests that the returned reader
// implements io.ReadCloser regardless of whether the
// underlying writer does.
func TestNewReader_Closer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pb := progress.New(ctx, os.Stdout, time.Millisecond, progress.DefaultColors())
	ctx = progress.NewContext(ctx, pb)
	defer pb.Stop()

	// bytes.Buffer doesn't implement io.Closer
	buf := &bytes.Buffer{}
	gotReader := progress.NewReader(ctx, "no closer", -1, buf)
	require.NotNil(t, gotReader)
	_, isCloser := gotReader.(io.ReadCloser)
	assert.True(t, isCloser, "expected reader to be io.ReadCloser but was %T",
		gotReader)

	bufCloser := io.NopCloser(buf)
	gotReader = progress.NewReader(ctx, "closer", -1, bufCloser)
	require.NotNil(t, gotReader)
	_, isCloser = gotReader.(io.ReadCloser)
	assert.True(t, isCloser, "expected reader to be io.ReadCloser but was %T",
		gotReader)
}
