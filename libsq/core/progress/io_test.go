package progress_test

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/progress"
)

// newTestProgress returns a context carrying a Progress that writes to
// io.Discard. The Progress is stopped via t.Cleanup.
func newTestProgress(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	pb := progress.New(ctx, io.Discard, progress.DefaultMaxBars, time.Millisecond, nil)
	t.Cleanup(pb.Stop)
	return progress.NewContext(ctx, pb)
}

// errReadWriter returns wantErr on every Read/Write.
type errReadWriter struct {
	wantErr error
}

func (e errReadWriter) Write([]byte) (int, error) { return 0, e.wantErr }
func (e errReadWriter) Read([]byte) (int, error)  { return 0, e.wantErr }

// trackCloser records whether Close was called.
type trackCloser struct {
	io.Writer
	closed bool
}

func (c *trackCloser) Close() error {
	c.closed = true
	return nil
}

// plainWriter implements io.Writer but deliberately NOT io.ReaderFrom, to
// exercise the non-ReaderFrom branch of progCopier.ReadFrom.
type plainWriter struct {
	buf *bytes.Buffer
}

func (w plainWriter) Write(p []byte) (int, error) { return w.buf.Write(p) }

func TestWriter_Write(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	buf := &bytes.Buffer{}
	w := progress.NewWriter(ctx, "write", -1, buf)

	n, err := w.Write([]byte("hello"))
	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "hello", buf.String())

	require.NoError(t, w.Close())
}

func TestWriter_Write_ctxCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = newTestProgress(t, ctx)
	w := progress.NewWriter(ctx, "write", -1, &bytes.Buffer{})

	cancel()
	n, err := w.Write([]byte("hello"))
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, n)
}

func TestWriter_Write_underlyingErr(t *testing.T) {
	t.Parallel()

	wantErr := io.ErrShortWrite
	ctx := newTestProgress(t, context.Background())
	w := progress.NewWriter(ctx, "write", -1, errReadWriter{wantErr: wantErr})

	_, err := w.Write([]byte("hello"))
	require.ErrorIs(t, err, wantErr)
}

func TestWriter_Close_underlyingCloser(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	tc := &trackCloser{Writer: &bytes.Buffer{}}
	w := progress.NewWriter(ctx, "write", -1, tc)

	require.NoError(t, w.Close())
	require.True(t, tc.closed, "underlying closer should have been closed")
}

func TestWriter_Close_ctxCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = newTestProgress(t, ctx)
	w := progress.NewWriter(ctx, "write", -1, &bytes.Buffer{})

	cancel()
	err := w.Close()
	require.ErrorIs(t, err, context.Canceled)
}

func TestWriter_Stop(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	buf := &bytes.Buffer{}
	w := progress.NewWriter(ctx, "write", -1, buf)

	// Stop should remove the bar but not prevent further writing.
	w.Stop()
	n, err := w.Write([]byte("after stop"))
	require.NoError(t, err)
	require.Equal(t, 10, n)
}

func TestWriter_idempotentWrap(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	w := progress.NewWriter(ctx, "write", -1, &bytes.Buffer{})

	// Wrapping the same writer with the same ctx should return it unchanged.
	w2 := progress.NewWriter(ctx, "write", -1, w)
	require.Same(t, w, w2)
}

func TestWriter_noProgressInContext(t *testing.T) {
	t.Parallel()

	// No Progress in context: NewWriter delegates to contextio, but the result
	// must still be a functional progress.Writer (io.WriteCloser + Stop).
	buf := &bytes.Buffer{}
	w := progress.NewWriter(context.Background(), "write", -1, buf)

	n, err := w.Write([]byte("delegated"))
	require.NoError(t, err)
	require.Equal(t, 9, n)
	require.Equal(t, "delegated", buf.String())

	w.Stop() // no-op, must not panic
	require.NoError(t, w.Close())
}

func TestWriter_noProgressInContext_underlyingCloser(t *testing.T) {
	t.Parallel()

	tc := &trackCloser{Writer: &bytes.Buffer{}}
	w := progress.NewWriter(context.Background(), "write", -1, tc)
	require.NoError(t, w.Close())
	require.True(t, tc.closed)
}

func TestReader_Read(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	r := progress.NewReader(ctx, "read", 5, strings.NewReader("hello"))

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
}

func TestReader_Read_ctxCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = newTestProgress(t, ctx)
	r := progress.NewReader(ctx, "read", -1, strings.NewReader("hello"))

	cancel()
	n, err := r.Read(make([]byte, 8))
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, n)
}

func TestReader_Read_underlyingErr(t *testing.T) {
	t.Parallel()

	wantErr := io.ErrUnexpectedEOF
	ctx := newTestProgress(t, context.Background())
	r := progress.NewReader(ctx, "read", -1, errReadWriter{wantErr: wantErr})

	_, err := r.Read(make([]byte, 8))
	require.ErrorIs(t, err, wantErr)
}

func TestReader_Close_underlyingCloser(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	rc := io.NopCloser(strings.NewReader("hello"))
	r := progress.NewReader(ctx, "read", -1, rc)

	rcloser, ok := r.(io.ReadCloser)
	require.True(t, ok)
	require.NoError(t, rcloser.Close())
}

func TestReader_idempotentWrap(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	r := progress.NewReader(ctx, "read", -1, strings.NewReader("hello"))

	r2 := progress.NewReader(ctx, "read", -1, r)
	require.Same(t, r, r2)
}

func TestReader_noProgressInContext(t *testing.T) {
	t.Parallel()

	r := progress.NewReader(context.Background(), "read", -1, strings.NewReader("delegated"))
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "delegated", string(got))
}

// TestCopier_ReadFrom_readerFrom exercises the io.ReaderFrom branch of
// progCopier.ReadFrom, where the destination is itself an io.ReaderFrom
// (bytes.Buffer is).
func TestCopier_ReadFrom_readerFrom(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	dst := &bytes.Buffer{}
	w := progress.NewWriter(ctx, "copy", int64(len("payload")), dst)

	n, err := io.Copy(w, strings.NewReader("payload"))
	require.NoError(t, err)
	require.Equal(t, int64(7), n)
	require.Equal(t, "payload", dst.String())
}

// TestCopier_ReadFrom_notReaderFrom exercises the non-ReaderFrom branch of
// progCopier.ReadFrom, where the destination only implements io.Writer.
func TestCopier_ReadFrom_notReaderFrom(t *testing.T) {
	t.Parallel()

	ctx := newTestProgress(t, context.Background())
	buf := &bytes.Buffer{}
	w := progress.NewWriter(ctx, "copy", -1, plainWriter{buf: buf})

	n, err := io.Copy(w, strings.NewReader("payload"))
	require.NoError(t, err)
	require.Equal(t, int64(7), n)
	require.Equal(t, "payload", buf.String())
}

// TestCopier_ReadFrom_ctxCanceled exercises the context-cancellation path of
// the non-ReaderFrom branch of progCopier.ReadFrom.
func TestCopier_ReadFrom_ctxCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	ctx = newTestProgress(t, ctx)
	buf := &bytes.Buffer{}
	w := progress.NewWriter(ctx, "copy", -1, plainWriter{buf: buf})

	cancel()
	rf, ok := w.(io.ReaderFrom)
	require.True(t, ok)
	n, err := rf.ReadFrom(strings.NewReader("payload"))
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, n)
}
