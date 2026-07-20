package contextio_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
)

// plainWriter implements only io.Writer (no Close, no ReaderFrom).
type plainWriter struct{ w io.Writer }

func (p plainWriter) Write(b []byte) (int, error) { return p.w.Write(b) }

// trackCloser wraps a reader/writer and records Close, returning closeErr.
type trackCloser struct {
	io.Writer
	io.Reader
	closeErr error
	closed   bool
}

func (c *trackCloser) Close() error {
	c.closed = true
	return c.closeErr
}

// errWriter always fails Write with err.
type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

// readerFunc adapts a func to io.Reader.
type readerFunc func([]byte) (int, error)

func (f readerFunc) Read(p []byte) (int, error) { return f(p) }

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

	bufCloser := ioz.WriteCloser(buf)
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

func TestNewWriter_ReaderFrom(t *testing.T) {
	ctx := context.Background()

	// Non-closer writer: returned writer must implement io.ReaderFrom.
	w := contextio.NewWriter(ctx, &bytes.Buffer{})
	_, ok := w.(io.ReaderFrom)
	require.True(t, ok, "NewWriter(non-closer) must implement io.ReaderFrom, got %T", w)

	// Closer writer: returned writer must implement BOTH io.WriteCloser and
	// io.ReaderFrom (regression test for the copyCloser ReaderFrom gap).
	wc := contextio.NewWriter(ctx, ioz.WriteCloser(&bytes.Buffer{}))
	_, ok = wc.(io.WriteCloser)
	require.True(t, ok, "NewWriter(closer) must implement io.WriteCloser, got %T", wc)
	_, ok = wc.(io.ReaderFrom)
	require.True(t, ok, "NewWriter(closer) must implement io.ReaderFrom, got %T", wc)
}

func TestNewWriter_dedup(t *testing.T) {
	ctx := context.Background()

	// Wrapping an already-wrapped writer with the same ctx returns it as-is.
	w := contextio.NewWriter(ctx, &bytes.Buffer{})
	require.Same(t, w, contextio.NewWriter(ctx, w))

	wc := contextio.NewWriter(ctx, ioz.WriteCloser(&bytes.Buffer{}))
	require.Same(t, wc, contextio.NewWriter(ctx, wc))

	// A different ctx wraps again (new instance).
	other := contextio.NewWriter(context.TODO(), w)
	require.NotSame(t, w, other)
}

func TestNewReader_dedup(t *testing.T) {
	ctx := context.Background()

	r := contextio.NewReader(ctx, &bytes.Buffer{})
	require.Same(t, r, contextio.NewReader(ctx, r))

	rc := contextio.NewReader(ctx, io.NopCloser(&bytes.Buffer{}))
	require.Same(t, rc, contextio.NewReader(ctx, rc))

	other := contextio.NewReader(context.TODO(), r)
	require.NotSame(t, r, other)
}

func TestWriter_Write(t *testing.T) {
	t.Run("live", func(t *testing.T) {
		buf := &bytes.Buffer{}
		w := contextio.NewWriter(context.Background(), buf)
		n, err := w.Write([]byte("hello"))
		require.NoError(t, err)
		require.Equal(t, 5, n)
		require.Equal(t, "hello", buf.String())
	})

	t.Run("cancelled_before_write", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		buf := &bytes.Buffer{}
		w := contextio.NewWriter(ctx, buf)
		n, err := w.Write([]byte("hello"))
		require.Zero(t, n)
		require.ErrorIs(t, err, context.Canceled)
		require.Empty(t, buf.String(), "nothing should be written after cancellation")
	})

	t.Run("cancel_cause", func(t *testing.T) {
		sentinel := errors.New("my cause")
		ctx, cancel := context.WithCancelCause(context.Background())
		cancel(sentinel)
		w := contextio.NewWriter(ctx, &bytes.Buffer{})
		_, err := w.Write([]byte("x"))
		require.ErrorIs(t, err, sentinel, "cancel cause should be surfaced")
	})

	t.Run("underlying_error", func(t *testing.T) {
		wantErr := errors.New("disk full")
		w := contextio.NewWriter(context.Background(), errWriter{err: wantErr})
		_, err := w.Write([]byte("x"))
		require.ErrorIs(t, err, wantErr)
	})
}

func TestWriteCloser_Close(t *testing.T) {
	t.Run("live_returns_underlying", func(t *testing.T) {
		closeErr := errors.New("close boom")
		tc := &trackCloser{Writer: &bytes.Buffer{}, closeErr: closeErr}
		w := contextio.NewWriter(context.Background(), tc)
		wc := w.(io.WriteCloser)
		require.ErrorIs(t, wc.Close(), closeErr)
		require.True(t, tc.closed)
	})

	t.Run("cancelled_still_closes_returns_cause", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		tc := &trackCloser{Writer: &bytes.Buffer{}}
		wc := contextio.NewWriter(ctx, tc).(io.WriteCloser)
		err := wc.Close()
		require.ErrorIs(t, err, context.Canceled)
		require.True(t, tc.closed, "underlying must be closed even when ctx is done")
	})
}

func TestReader_Read(t *testing.T) {
	t.Run("live", func(t *testing.T) {
		r := contextio.NewReader(context.Background(), strings.NewReader("hello"))
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "hello", string(got))
	})

	t.Run("cancelled_before_read", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		r := contextio.NewReader(ctx, strings.NewReader("hello"))
		n, err := r.Read(make([]byte, 4))
		require.Zero(t, n)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("underlying_error", func(t *testing.T) {
		wantErr := errors.New("read boom")
		r := contextio.NewReader(context.Background(), ioz.ErrReader{Err: wantErr})
		_, err := r.Read(make([]byte, 4))
		require.ErrorIs(t, err, wantErr)
	})
}

func TestReadCloser_Close(t *testing.T) {
	t.Run("live_returns_underlying", func(t *testing.T) {
		closeErr := errors.New("rc close boom")
		tc := &trackCloser{Reader: strings.NewReader("x"), closeErr: closeErr}
		rc := contextio.NewReader(context.Background(), tc).(io.ReadCloser)
		require.ErrorIs(t, rc.Close(), closeErr)
		require.True(t, tc.closed)
	})

	t.Run("cancelled_still_closes", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		tc := &trackCloser{Reader: strings.NewReader("x")}
		rc := contextio.NewReader(ctx, tc).(io.ReadCloser)
		require.ErrorIs(t, rc.Close(), context.Canceled)
		require.True(t, tc.closed)
	})
}

func TestCopier_ReadFrom(t *testing.T) {
	const src = "the quick brown fox"

	t.Run("dst_is_readerfrom", func(t *testing.T) {
		// bytes.Buffer implements io.ReaderFrom.
		buf := &bytes.Buffer{}
		rf := contextio.NewWriter(context.Background(), buf).(io.ReaderFrom)
		n, err := rf.ReadFrom(strings.NewReader(src))
		require.NoError(t, err)
		require.Equal(t, int64(len(src)), n)
		require.Equal(t, src, buf.String())
	})

	t.Run("dst_not_readerfrom", func(t *testing.T) {
		// plainWriter is not a ReaderFrom, exercising the io.Copy(&writer, r) path.
		buf := &bytes.Buffer{}
		rf := contextio.NewWriter(context.Background(), plainWriter{buf}).(io.ReaderFrom)
		n, err := rf.ReadFrom(strings.NewReader(src))
		require.NoError(t, err)
		require.Equal(t, int64(len(src)), n)
		require.Equal(t, src, buf.String())
	})

	t.Run("cancelled_readerfrom", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		rf := contextio.NewWriter(ctx, &bytes.Buffer{}).(io.ReaderFrom)
		_, err := rf.ReadFrom(strings.NewReader(src))
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("cancelled_not_readerfrom", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		buf := &bytes.Buffer{}
		rf := contextio.NewWriter(ctx, plainWriter{buf}).(io.ReaderFrom)
		n, err := rf.ReadFrom(strings.NewReader(src))
		require.Zero(t, n)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("closer_dst_readfrom", func(t *testing.T) {
		// The copyCloser path must also support context-aware ReadFrom.
		buf := &bytes.Buffer{}
		wc := contextio.NewWriter(context.Background(), ioz.WriteCloser(buf))
		rf := wc.(io.ReaderFrom)
		n, err := rf.ReadFrom(strings.NewReader(src))
		require.NoError(t, err)
		require.Equal(t, int64(len(src)), n)
		require.Equal(t, src, buf.String())
	})

	t.Run("underlying_returns_ctx_error_resolves_to_cause", func(t *testing.T) {
		// Exercises the err == ctx.Err() branch of cause(): when the underlying
		// op returns the bare context error, ReadFrom must resolve it to the
		// context's cause. Using WithCancelCause with a distinct sentinel makes
		// the assertion meaningful: it would fail if cause() returned the raw
		// context.Canceled instead of the cause.
		ctx, cancel := context.WithCancelCause(context.Background())
		sentinel := errors.New("my cause")
		// ctx is live at ReadFrom entry; the reader then cancels it and returns
		// the bare context.Canceled (== ctx.Err()), forcing the cause() branch.
		r := readerFunc(func([]byte) (int, error) {
			cancel(sentinel)
			return 0, context.Canceled
		})
		rf := contextio.NewWriter(ctx, plainWriter{&bytes.Buffer{}}).(io.ReaderFrom)
		_, err := rf.ReadFrom(r)
		require.ErrorIs(t, err, sentinel, "bare ctx error must resolve to the cause")
	})
}

func TestNewCloser(t *testing.T) {
	t.Run("live_returns_underlying", func(t *testing.T) {
		closeErr := errors.New("closer boom")
		tc := &trackCloser{closeErr: closeErr}
		c := contextio.NewCloser(context.Background(), tc)
		require.ErrorIs(t, c.Close(), closeErr)
		require.True(t, tc.closed)
	})

	t.Run("cancelled_still_closes_returns_cause", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		tc := &trackCloser{}
		c := contextio.NewCloser(ctx, tc)
		require.ErrorIs(t, c.Close(), context.Canceled)
		require.True(t, tc.closed, "underlying closer must always be closed")
	})
}
