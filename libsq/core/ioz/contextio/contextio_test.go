package contextio_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewWriter_Closer tests that the returned writer
// implements io.WriteCloser, or not, depending upon the type of
// the underlying writer.
func TestNewWriter_Closer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// bytes.Buffer doesn't implement io.Closer
	buf := &bytes.Buffer{}
	gotWriter := contextio.NewWriter(ctx, buf)
	require.NotNil(t, gotWriter)
	_, isCloser := gotWriter.(io.WriteCloser)

	assert.False(t, isCloser, "expected reader NOT to be io.WriteCloser, but was %T",
		gotWriter)

	bufCloser := ioz.ToWriteCloser(buf)
	gotWriter = contextio.NewWriter(ctx, bufCloser)
	require.NotNil(t, gotWriter)
	_, isCloser = gotWriter.(io.WriteCloser)

	assert.True(t, isCloser, "expected reader to implement io.WriteCloser, but was %T",
		gotWriter)
}

// TestNewReader_Closer tests that the returned reader
// implements io.ReadCloser, or not, depending upon the type of
// the underlying writer.
func TestNewReader_Closer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// bytes.Buffer doesn't implement io.Closer
	buf := &bytes.Buffer{}
	gotReader := contextio.NewReader(ctx, buf)
	require.NotNil(t, gotReader)
	_, isCloser := gotReader.(io.ReadCloser)

	assert.False(t, isCloser, "expected reader NOT to be io.ReadCloser but was %T",
		gotReader)

	bufCloser := io.NopCloser(buf)
	gotReader = contextio.NewReader(ctx, bufCloser)
	require.NotNil(t, gotReader)
	_, isCloser = gotReader.(io.ReadCloser)

	assert.True(t, isCloser, "expected reader to be io.ReadCloser but was %T",
		gotReader)
}
