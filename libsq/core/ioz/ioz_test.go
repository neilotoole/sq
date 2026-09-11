package ioz_test

import (
	"bytes"
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

	testCases := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "nonempty", input: "In Xanadu did Kubla Khan"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rdr := ioz.NewErrorAfterBytesReader([]byte(tc.input), wantErr)
			got, err := io.ReadAll(rdr)
			require.Equal(t, tc.input, string(got))
			require.True(t, errors.Is(err, wantErr))

			// A subsequent read continues to return the sentinel error.
			n, err := rdr.Read(make([]byte, 8))
			require.Zero(t, n)
			require.True(t, errors.Is(err, wantErr))
		})
	}
}

func TestNewErrorAfterBytesReader_partialReads(t *testing.T) {
	wantErr := errors.New("boom")
	const input = "abcdefghij" // 10 bytes
	rdr := ioz.NewErrorAfterBytesReader([]byte(input), wantErr)

	// Read with a small buffer to exercise the partial-read path, where the
	// sentinel error is only returned once the final byte has been consumed.
	var got []byte
	buf := make([]byte, 4)
	for {
		n, err := rdr.Read(buf)
		got = append(got, buf[:n]...)
		if err != nil {
			require.True(t, errors.Is(err, wantErr))
			break
		}
	}
	require.Equal(t, input, string(got))
}

func TestNewErrorAfterBytesReader_nilErrPanics(t *testing.T) {
	require.Panics(t, func() {
		ioz.NewErrorAfterBytesReader([]byte("x"), nil)
	})
}

// TestNewErrorAfterBytesReader_concurrent hammers Read from multiple goroutines.
// Under -race it would catch the missing mutex on errorAfterBytesReader.count.
func TestNewErrorAfterBytesReader_concurrent(t *testing.T) {
	wantErr := errors.New("done")
	rdr := ioz.NewErrorAfterBytesReader([]byte("abcdefghij"), wantErr)

	wg := &sync.WaitGroup{}
	for range 8 {
		wg.Go(func() {
			buf := make([]byte, 4)
			for {
				if _, err := rdr.Read(buf); err != nil {
					return
				}
			}
		})
	}
	wg.Wait()

	// Once exhausted, further reads return the sentinel error.
	_, err := rdr.Read(make([]byte, 4))
	require.True(t, errors.Is(err, wantErr))
}

func TestNewErrorAfterRandNReader(t *testing.T) {
	wantErr := errors.New("kaboom")

	t.Run("read_n_then_err", func(t *testing.T) {
		const afterN = 10
		rdr := ioz.NewErrorAfterRandNReader(afterN, wantErr)
		got, err := io.ReadAll(rdr)
		require.Len(t, got, afterN)
		require.True(t, errors.Is(err, wantErr))
	})

	t.Run("zero_n_errs_immediately", func(t *testing.T) {
		rdr := ioz.NewErrorAfterRandNReader(0, wantErr)
		n, err := rdr.Read(make([]byte, 8))
		require.Zero(t, n)
		require.True(t, errors.Is(err, wantErr))
	})

	t.Run("small_buffer_partial_reads", func(t *testing.T) {
		// With a buffer smaller than the remaining allowance, Read returns a
		// full buffer with a nil error (the allowed > len(p) branch).
		const afterN = 10
		rdr := ioz.NewErrorAfterRandNReader(afterN, wantErr)
		buf := make([]byte, 4)
		var total int
		var lastErr error
		for {
			n, err := rdr.Read(buf)
			total += n
			if err != nil {
				lastErr = err
				break
			}
		}
		require.Equal(t, afterN, total)
		require.True(t, errors.Is(lastErr, wantErr))
	})
}

func TestClose(t *testing.T) {
	ctx := context.Background()

	// Nil closer is a no-op.
	require.NotPanics(t, func() { ioz.Close(ctx, nil) })

	// Nil context is tolerated.
	var nilCtx context.Context
	require.NotPanics(t, func() { ioz.Close(nilCtx, &errCloser{}) })

	// A closer that returns an error is logged, not panicked.
	require.NotPanics(t, func() { ioz.Close(ctx, &errCloser{err: errors.New("close boom")}) })

	// A closer that returns nil.
	require.NotPanics(t, func() { ioz.Close(ctx, &errCloser{}) })
}

func TestCopyAsync(t *testing.T) {
	t.Parallel()

	src := "the quick brown fox"
	dst := &bytes.Buffer{}

	// Capture the callback args and assert on them from the test goroutine;
	// calling require.* inside the callback goroutine would invoke t.FailNow on
	// the wrong goroutine. Receiving from done establishes a happens-before edge
	// with the copy, so dst is safe to read afterwards.
	var (
		gotWritten int64
		gotErr     error
	)
	done := make(chan struct{})
	ioz.CopyAsync(dst, strings.NewReader(src), func(written int64, err error) {
		gotWritten = written
		gotErr = err
		close(done)
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("CopyAsync callback not invoked")
	}
	require.NoError(t, gotErr)
	require.Equal(t, int64(len(src)), gotWritten)
	require.Equal(t, src, dst.String())
}

func TestCopyAsync_nilCallback(t *testing.T) {
	t.Parallel()

	// With a nil callback there's no completion signal, so synchronize on the
	// source reaching EOF, which happens-after the final write to dst. This also
	// confirms a nil callback doesn't panic.
	//
	// dst is wrapped in plainWriter so io.Copy uses the Read/Write loop rather
	// than bytes.Buffer.ReadFrom; ReadFrom writes bookkeeping fields after the
	// final read returns (i.e. after EOF fires close(done)), which would race
	// with the buf.String() read below.
	//
	// The source is followed by an EmptyReader so io.EOF always arrives on a
	// final zero-byte read, strictly after the last write to buf. An io.Reader
	// may legally return (n>0, io.EOF) in a single call; appending EmptyReader
	// removes any dependence on the source deferring EOF to a separate read.
	const src = "the quick brown fox"
	buf := &bytes.Buffer{}
	done := make(chan struct{})
	srcRdr := io.MultiReader(strings.NewReader(src), ioz.EmptyReader{})
	r := ioz.NotifyOnEOFReader(srcRdr, func(err error) error {
		close(done)
		return err
	})

	ioz.CopyAsync(plainWriter{buf}, r, nil)

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("CopyAsync did not complete")
	}
	require.Equal(t, src, buf.String())
}

func TestMarshalUnmarshalYAML_RoundTrip(t *testing.T) {
	type config struct {
		Name  string `yaml:"name"`
		Count int    `yaml:"count"`
	}
	want := config{Name: "xanadu", Count: 7}

	b, err := ioz.MarshalYAML(want)
	require.NoError(t, err)
	require.Contains(t, string(b), "name: xanadu")

	var got config
	require.NoError(t, ioz.UnmarshallYAML(b, &got))
	require.Equal(t, want, got)
}

func TestMarshalYAML_error(t *testing.T) {
	// A channel cannot be marshaled to YAML.
	_, err := ioz.MarshalYAML(make(chan int))
	require.Error(t, err)
}

func TestUnmarshallYAML_error(t *testing.T) {
	var v map[string]any
	err := ioz.UnmarshallYAML([]byte("\tnot: valid: yaml: ["), &v)
	require.Error(t, err)
}

func TestDelayReader_nil(t *testing.T) {
	require.Nil(t, ioz.DelayReader(nil, time.Millisecond, false))
}

func TestDelayReader_noJitter(t *testing.T) {
	r := ioz.DelayReader(strings.NewReader("hello"), 0, false)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))

	// A plain reader (non-Closer) must not yield a Closer.
	_, ok := r.(io.Closer)
	require.False(t, ok)
}

func TestDelayReader_zeroDelayWithJitter(t *testing.T) {
	// jitter with a zero delay must not panic (mrand.Int63n panics for n <= 0).
	r := ioz.DelayReader(strings.NewReader("hello"), 0, true)
	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))
}

func TestDelayReader_closerPassthrough(t *testing.T) {
	closeErr := errors.New("delay close boom")
	trc := &trackReadCloser{Reader: strings.NewReader("data"), closeErr: closeErr}
	r := ioz.DelayReader(trc, 0, false)

	c, ok := r.(io.Closer)
	require.True(t, ok, "reader wrapping a Closer must implement Closer")

	got, err := io.ReadAll(r)
	require.NoError(t, err)
	require.Equal(t, "data", string(got))

	require.Equal(t, closeErr, c.Close())
	require.True(t, trc.closed)
}

func TestLimitRandReader(t *testing.T) {
	const limit = 128
	got, err := io.ReadAll(ioz.LimitRandReader(limit))
	require.NoError(t, err)
	require.Len(t, got, limit)
}

func TestNotifyOnceWriter(t *testing.T) {
	var calls int
	buf := &bytes.Buffer{}
	w := ioz.NotifyOnceWriter(buf, func() { calls++ })

	_, err := w.Write([]byte("a"))
	require.NoError(t, err)
	_, err = w.Write([]byte("b"))
	require.NoError(t, err)

	require.Equal(t, 1, calls, "notify fn must fire exactly once")
	require.Equal(t, "ab", buf.String())

	// Nil w or nil fn returns w unchanged.
	require.Equal(t, buf, ioz.NotifyOnceWriter(buf, nil))
	require.Nil(t, ioz.NotifyOnceWriter(nil, func() {}))
}

func TestNotifyWriter(t *testing.T) {
	var got []int
	buf := &bytes.Buffer{}
	w := ioz.NotifyWriter(buf, func(n int) { got = append(got, n) })

	_, err := w.Write([]byte("abc"))
	require.NoError(t, err)
	_, err = w.Write(nil) // zero-length write still notifies
	require.NoError(t, err)
	_, err = w.Write([]byte("de"))
	require.NoError(t, err)

	require.Equal(t, []int{3, 0, 2}, got)
	require.Equal(t, "abcde", buf.String())

	require.Equal(t, buf, ioz.NotifyWriter(buf, nil))
	require.Nil(t, ioz.NotifyWriter(nil, func(int) {}))
}

func TestNotifyOnEOFReader(t *testing.T) {
	t.Run("non_closer_transforms_eof", func(t *testing.T) {
		sentinel := errors.New("transformed")
		var called bool
		r := ioz.NotifyOnEOFReader(strings.NewReader("hi"), func(err error) error {
			called = true
			require.True(t, errors.Is(err, io.EOF))
			return sentinel
		})
		_, ok := r.(io.Closer)
		require.False(t, ok)

		got, err := io.ReadAll(r)
		require.Equal(t, "hi", string(got))
		require.True(t, called)
		require.True(t, errors.Is(err, sentinel))
	})

	t.Run("closer_variant", func(t *testing.T) {
		var called bool
		trc := &trackReadCloser{Reader: strings.NewReader("hi")}
		r := ioz.NotifyOnEOFReader(trc, func(err error) error {
			called = true
			return err
		})
		c, ok := r.(io.ReadCloser)
		require.True(t, ok)

		got, err := io.ReadAll(r)
		require.Equal(t, "hi", string(got))
		require.True(t, called)
		require.NoError(t, err, "io.ReadAll swallows the EOF returned by fn")

		require.NoError(t, c.Close())
		require.True(t, trc.closed)
	})

	t.Run("nil_args", func(t *testing.T) {
		require.Nil(t, ioz.NotifyOnEOFReader(nil, func(error) error { return nil }))
		r := strings.NewReader("x")
		require.Equal(t, io.Reader(r), ioz.NotifyOnEOFReader(r, nil))
	})
}

func TestNotifyOnErrorReader(t *testing.T) {
	t.Run("transforms_error", func(t *testing.T) {
		srcErr := errors.New("source boom")
		sentinel := errors.New("transformed")
		var called bool
		r := ioz.NotifyOnErrorReader(ioz.ErrReader{Err: srcErr}, func(err error) error {
			called = true
			require.True(t, errors.Is(err, srcErr))
			return sentinel
		})
		_, err := io.ReadAll(r)
		require.True(t, called)
		require.True(t, errors.Is(err, sentinel))
	})

	t.Run("eof_counts_as_error_and_invokes_fn", func(t *testing.T) {
		var called bool
		r := ioz.NotifyOnErrorReader(strings.NewReader("ok"), func(err error) error {
			called = true
			return err
		})
		// The underlying strings.Reader returns io.EOF, which counts as an error,
		// so fn is invoked. Verify the bytes still arrive intact.
		got, err := io.ReadAll(r)
		require.Equal(t, "ok", string(got))
		require.True(t, called, "fn must fire on io.EOF")
		require.NoError(t, err, "io.ReadAll swallows the EOF returned by fn")
	})

	t.Run("closer_variant", func(t *testing.T) {
		trc := &trackReadCloser{Reader: strings.NewReader("hi")}
		r := ioz.NotifyOnErrorReader(trc, func(err error) error { return err })
		c, ok := r.(io.ReadCloser)
		require.True(t, ok)
		require.NoError(t, c.Close())
		require.True(t, trc.closed)
	})

	t.Run("nil_args", func(t *testing.T) {
		require.Nil(t, ioz.NotifyOnErrorReader(nil, func(error) error { return nil }))
		r := strings.NewReader("x")
		require.Equal(t, io.Reader(r), ioz.NotifyOnErrorReader(r, nil))
	})
}

func TestWrittenWriter(t *testing.T) {
	t.Run("tracks_bytes", func(t *testing.T) {
		buf := &bytes.Buffer{}
		w := &ioz.WrittenWriter{W: buf}
		n, err := w.Write([]byte("abc"))
		require.NoError(t, err)
		require.Equal(t, 3, n)
		n, err = w.Write([]byte("de"))
		require.NoError(t, err)
		require.Equal(t, 2, n)
		require.Equal(t, int64(5), w.Written)
		require.Equal(t, "abcde", buf.String())
	})

	t.Run("error_latches", func(t *testing.T) {
		wantErr := errors.New("write boom")
		w := &ioz.WrittenWriter{W: errWriter{err: wantErr}}
		n, err := w.Write([]byte("abc"))
		require.Zero(t, n)
		require.True(t, errors.Is(err, wantErr))
		require.True(t, errors.Is(w.Err, wantErr))

		// Subsequent writes are no-op and return the stored error.
		n, err = w.Write([]byte("def"))
		require.Zero(t, n)
		require.True(t, errors.Is(err, wantErr))
	})
}

func TestWriteCloser(t *testing.T) {
	t.Run("passthrough_existing_writecloser", func(t *testing.T) {
		wc := nopWC{&bytes.Buffer{}}
		require.Equal(t, io.WriteCloser(wc), ioz.WriteCloser(wc))
	})

	t.Run("wraps_plain_writer", func(t *testing.T) {
		w := plainWriter{&bytes.Buffer{}}
		wc := ioz.WriteCloser(w)
		_, isReaderFrom := wc.(io.ReaderFrom)
		require.False(t, isReaderFrom)
		_, err := wc.Write([]byte("hi"))
		require.NoError(t, err)
		require.NoError(t, wc.Close())
	})

	t.Run("preserves_readerfrom", func(t *testing.T) {
		// bytes.Buffer implements io.ReaderFrom but not io.Closer.
		buf := &bytes.Buffer{}
		wc := ioz.WriteCloser(buf)
		rf, ok := wc.(io.ReaderFrom)
		require.True(t, ok, "ReaderFrom must be preserved")

		n, err := rf.ReadFrom(strings.NewReader("xanadu"))
		require.NoError(t, err)
		require.Equal(t, int64(6), n)
		require.Equal(t, "xanadu", buf.String())
		require.NoError(t, wc.Close())
	})
}

func TestDrainClose(t *testing.T) {
	t.Run("happy", func(t *testing.T) {
		trc := &trackReadCloser{Reader: strings.NewReader("abcdef")}
		n, err := ioz.DrainClose(trc)
		require.NoError(t, err)
		require.Equal(t, 6, n)
		require.True(t, trc.closed)
	})

	t.Run("drain_error_wins_over_close_error", func(t *testing.T) {
		drainErr := errors.New("drain boom")
		closeErr := errors.New("close boom")
		trc := &trackReadCloser{Reader: ioz.ErrReader{Err: drainErr}, closeErr: closeErr}
		_, err := ioz.DrainClose(trc)
		require.True(t, errors.Is(err, drainErr))
		require.False(t, errors.Is(err, closeErr))
		require.True(t, trc.closed)
	})

	t.Run("close_error_when_drain_ok", func(t *testing.T) {
		closeErr := errors.New("close boom")
		trc := &trackReadCloser{Reader: strings.NewReader("abc"), closeErr: closeErr}
		n, err := ioz.DrainClose(trc)
		require.Equal(t, 3, n)
		require.True(t, errors.Is(err, closeErr))
	})
}

func TestReadCloserNotifier(t *testing.T) {
	t.Run("close_once_and_notifies", func(t *testing.T) {
		closeErr := errors.New("close boom")
		var gotErr error
		var calls int
		trc := &trackReadCloser{Reader: strings.NewReader("hi"), closeErr: closeErr}
		rc := ioz.ReadCloserNotifier(trc, func(err error) {
			calls++
			gotErr = err
		})

		require.True(t, errors.Is(rc.Close(), closeErr))
		// A second Close is a no-op that returns the same error.
		require.True(t, errors.Is(rc.Close(), closeErr))
		require.Equal(t, 1, calls)
		require.True(t, errors.Is(gotErr, closeErr))
	})

	t.Run("nil_args", func(t *testing.T) {
		require.Nil(t, ioz.ReadCloserNotifier(nil, func(error) {}))
		trc := &trackReadCloser{Reader: strings.NewReader("hi")}
		require.Equal(t, io.ReadCloser(trc), ioz.ReadCloserNotifier(trc, nil))
	})
}

func TestEmptyReader(t *testing.T) {
	got, err := io.ReadAll(ioz.EmptyReader{})
	require.NoError(t, err)
	require.Empty(t, got)
}

func TestErrReader(t *testing.T) {
	wantErr := errors.New("always")
	n, err := ioz.ErrReader{Err: wantErr}.Read(make([]byte, 4))
	require.Zero(t, n)
	require.True(t, errors.Is(err, wantErr))
}

func TestWriteErrorCloser(t *testing.T) {
	t.Run("invokes_fn", func(t *testing.T) {
		wantErr := errors.New("upstream boom")
		var gotErr error
		wec := ioz.NewFuncWriteErrorCloser(ioz.WriteCloser(&bytes.Buffer{}), func(err error) {
			gotErr = err
		})
		n, err := wec.Write([]byte("hi"))
		require.NoError(t, err)
		require.Equal(t, 2, n)
		wec.Error(wantErr)
		require.True(t, errors.Is(gotErr, wantErr))
		require.NoError(t, wec.Close())
	})

	t.Run("nil_fn", func(t *testing.T) {
		wec := ioz.NewFuncWriteErrorCloser(ioz.WriteCloser(&bytes.Buffer{}), nil)
		require.NotPanics(t, func() { wec.Error(errors.New("x")) })
	})
}

// --- test helpers ---

type errCloser struct{ err error }

func (c *errCloser) Close() error { return c.err }

type errWriter struct{ err error }

func (w errWriter) Write([]byte) (int, error) { return 0, w.err }

// trackReadCloser wraps a reader and records whether Close was called,
// returning closeErr from Close.
type trackReadCloser struct {
	io.Reader
	closeErr error
	closed   bool
}

func (c *trackReadCloser) Close() error {
	c.closed = true
	return c.closeErr
}

// nopWC is an io.WriteCloser whose Close is a no-op.
type nopWC struct{ io.Writer }

func (nopWC) Close() error { return nil }

// plainWriter implements only io.Writer (no Close, no ReaderFrom): embedding the
// io.Writer interface promotes Write but not ReadFrom, matching nopWC's idiom.
type plainWriter struct{ io.Writer }
