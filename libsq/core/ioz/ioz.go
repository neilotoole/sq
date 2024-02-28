// Package ioz contains supplemental io functionality.
package ioz

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"errors"
	"fmt"
	"io"
	mrand "math/rand"
	"os"
	"sync"
	"time"

	yaml "github.com/goccy/go-yaml"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
)

// RWPerms is the default file mode used for creating files.
const RWPerms = os.FileMode(0o600)

// Close is a convenience function to close c, logging a warning
// if c.Close returns an error. This is useful in defer, e.g.
//
//	defer ioz.Close(ctx, c)
func Close(ctx context.Context, c io.Closer) {
	if c == nil {
		return
	}

	err := c.Close()
	if ctx == nil {
		return
	}

	log := lg.FromContext(ctx)
	lg.WarnIfError(log, "Close", err)
}

// CopyAsync asynchronously copies from r to w, invoking
// non-nil callback when done.
func CopyAsync(w io.Writer, r io.Reader, callback func(written int64, err error)) {
	go func() {
		written, err := io.Copy(w, r)
		if callback != nil {
			callback(written, err)
		}
	}()
}

// marshalYAMLTo is our standard mechanism for encoding YAML.
func marshalYAMLTo(w io.Writer, v any) (err error) {
	// We copy our indent style from kubectl.
	// - 2 spaces
	// - Don't indent sequences.
	const yamlIndent = 2

	enc := yaml.NewEncoder(w,
		yaml.Indent(yamlIndent),
		yaml.IndentSequence(false),
		yaml.UseSingleQuote(false))
	if err = enc.Encode(v); err != nil {
		return errz.Wrap(err, "failed to encode YAML")
	}

	if err = enc.Close(); err != nil {
		return errz.Wrap(err, "close YAML encoder")
	}

	return nil
}

// MarshalYAML is our standard mechanism for encoding YAML.
func MarshalYAML(v any) ([]byte, error) {
	buf := &bytes.Buffer{}
	if err := marshalYAMLTo(buf, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// UnmarshallYAML is our standard mechanism for decoding YAML.
func UnmarshallYAML(data []byte, v any) error {
	return errz.Err(yaml.Unmarshal(data, v))
}

var _ io.Reader = (*delayReader)(nil)

// DelayReader returns an io.Reader that delays on each read from r.
// This is primarily intended for testing.
// If jitter is true, a randomized jitter factor is added to the delay.
// If r implements io.Closer, the returned reader will also
// implement io.Closer; if r doesn't implement io.Closer,
// the returned reader will not implement io.Closer.
// If r is nil, nil is returned.
func DelayReader(r io.Reader, delay time.Duration, jitter bool) io.Reader {
	if r == nil {
		return nil
	}

	dr := delayReader{r: r, delay: delay, jitter: jitter}
	if _, ok := r.(io.Closer); ok {
		return delayReadCloser{dr}
	}
	return dr
}

var _ io.Reader = (*delayReader)(nil)

type delayReader struct {
	r      io.Reader
	delay  time.Duration
	jitter bool
}

// Read implements io.Reader.
func (d delayReader) Read(p []byte) (n int, err error) {
	delay := d.delay
	if d.jitter {
		delay += time.Duration(mrand.Int63n(int64(d.delay))) / 3 //nolint:gosec
	}

	time.Sleep(delay)
	return d.r.Read(p)
}

var _ io.ReadCloser = (*delayReadCloser)(nil)

type delayReadCloser struct {
	delayReader
}

// Close implements io.Closer.
func (d delayReadCloser) Close() error {
	if c, ok := d.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// LimitRandReader returns an io.Reader that reads up to limit bytes
// from crypto/rand.Reader.
func LimitRandReader(limit int64) io.Reader {
	return io.LimitReader(crand.Reader, limit)
}

// NotifyOnceWriter returns an io.Writer that invokes fn before the first
// invocation of Write. If w or fn is nil, w is returned.
//
// See also: [NotifyWriter], which is a generalization of [NotifyOnceWriter].
func NotifyOnceWriter(w io.Writer, fn func()) io.Writer {
	if w == nil || fn == nil {
		return w
	}

	return &notifyOnceWriter{
		fn: fn,
		w:  w,
	}
}

var _ io.Writer = (*notifyOnceWriter)(nil)

type notifyOnceWriter struct {
	w          io.Writer
	fn         func()
	notifyOnce sync.Once
}

// Write implements io.Writer. On the first invocation of this method, the
// notify function is invoked, blocking until it returns. Subsequent invocations
// of Write don't trigger the notify function.
func (w *notifyOnceWriter) Write(p []byte) (n int, err error) {
	w.notifyOnce.Do(func() {
		w.fn()
	})

	return w.w.Write(p)
}

// NotifyWriter returns an [io.Writer] that invokes fn(n) before every
// invocation of Write, where the n arg to fn is the length of the byte slice to
// be written (which may be zero). If w or fn is nil, w is returned.
//
// See also: [NotifyOnceWriter].
func NotifyWriter(w io.Writer, fn func(n int)) io.Writer {
	if w == nil || fn == nil {
		return w
	}

	return &notifyWriter{
		fn: fn,
		w:  w,
	}
}

type notifyWriter struct {
	fn func(n int)
	w  io.Writer
}

// Write invokes notifyWriter.fn with len(p) before passing through the Write
// call to notifyWriter.w.
func (w *notifyWriter) Write(p []byte) (n int, err error) {
	w.fn(len(p))
	return w.w.Write(p)
}

// NotifyOnEOFReader returns an [io.Reader] that invokes fn
// when r.Read returns [io.EOF]. The error that fn returns is
// what's returned to the r caller: fn can transform the error
// or return it unchanged. If r or fn is nil, r is returned.
//
// If r is an [io.ReadCloser], the returned reader will also
// implement [io.ReadCloser].
//
// See also: [NotifyOnErrorReader], which is a generalization of
// [NotifyOnEOFReader].
func NotifyOnEOFReader(r io.Reader, fn func(error) error) io.Reader {
	if r == nil || fn == nil {
		return r
	}

	if rc, ok := r.(io.ReadCloser); ok {
		return &notifyOnEOFReadCloser{notifyOnEOFReader{r: rc, fn: fn}}
	}

	return &notifyOnEOFReader{r: r}
}

type notifyOnEOFReader struct {
	r  io.Reader
	fn func(error) error
}

// Read implements io.Reader.
func (r *notifyOnEOFReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	if err != nil && errors.Is(err, io.EOF) {
		err = r.fn(err)
	}

	return n, err
}

var _ io.ReadCloser = (*notifyOnEOFReadCloser)(nil)

type notifyOnEOFReadCloser struct {
	notifyOnEOFReader
}

// Close implements io.Closer.
func (r *notifyOnEOFReadCloser) Close() error {
	if c, ok := r.r.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// NotifyOnErrorReader returns an [io.Reader] that invokes fn
// when r.Read returns an error. The error that fn returns is
// what's returned to the r caller: fn can transform the error
// or return it unchanged. If r or fn is nil, r is returned.
//
// See also: [NotifyOnEOFReader], which is a specialization of
// [NotifyOnErrorReader].
func NotifyOnErrorReader(r io.Reader, fn func(error) error) io.Reader {
	if r == nil || fn == nil {
		return r
	}

	return &notifyOnErrorReader{r: r}
}

type notifyOnErrorReader struct {
	r  io.Reader
	fn func(error) error
}

// Read implements io.Reader.
func (r *notifyOnErrorReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	if err != nil {
		err = r.fn(err)
	}

	return n, err
}

// WriteCloser returns w as an io.WriteCloser. If w implements
// io.WriteCloser, w is returned. Otherwise, w is wrapped in a
// no-op decorator that implements io.WriteCloser.
//
// WriteCloser is the missing sibling of io.NopCloser, which
// isn't implemented in stdlib. See: https://github.com/golang/go/issues/22823.
func WriteCloser(w io.Writer) io.WriteCloser {
	if wc, ok := w.(io.WriteCloser); ok {
		return wc
	}
	return toNopWriteCloser(w)
}

func toNopWriteCloser(w io.Writer) io.WriteCloser {
	if _, ok := w.(io.ReaderFrom); ok {
		return nopWriteCloserReaderFrom{w}
	}
	return nopWriteCloser{w}
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

type nopWriteCloserReaderFrom struct {
	io.Writer
}

func (nopWriteCloserReaderFrom) Close() error { return nil }

func (c nopWriteCloserReaderFrom) ReadFrom(r io.Reader) (int64, error) {
	return c.Writer.(io.ReaderFrom).ReadFrom(r)
}

// DrainClose drains rc, returning the number of bytes read, and any error.
// The reader is always closed, even if the drain operation returned an error.
// If both the drain and the close operations return non-nil errors, the drain
// error is returned.
func DrainClose(rc io.ReadCloser) (n int, err error) {
	var n64 int64
	n64, err = io.Copy(io.Discard, rc)
	n = int(n64)

	closeErr := rc.Close()
	if err == nil {
		err = closeErr
	}
	return n, errz.Err(err)
}

// ReadCloserNotifier returns a new io.ReadCloser that invokes fn
// after Close is called, passing along any error from Close.
// If rc or fn is nil, rc is returned. Note that any subsequent
// calls to Close are no-op, and return the same error (if any)
// as the first invocation of Close.
func ReadCloserNotifier(rc io.ReadCloser, fn func(closeErr error)) io.ReadCloser {
	if rc == nil || fn == nil {
		return rc
	}
	return &readCloserNotifier{ReadCloser: rc, fn: fn}
}

type readCloserNotifier struct {
	closeErr error
	io.ReadCloser
	fn   func(error)
	once sync.Once
}

func (c *readCloserNotifier) Close() error {
	c.once.Do(func() {
		c.closeErr = c.ReadCloser.Close()
		c.fn(c.closeErr)
	})
	return c.closeErr
}

var _ io.Reader = EmptyReader{}

// EmptyReader is an io.Reader whose Read methods always returns io.EOF.
type EmptyReader struct{}

// Read always returns (0, io.EOF).
func (e EmptyReader) Read([]byte) (n int, err error) {
	return 0, io.EOF
}

var _ io.Reader = (*ErrReader)(nil)

// ErrReader is an [io.Reader] that always returns an error.
type ErrReader struct {
	Err error
}

// Read implements [io.Reader]: it always returns [ErrReader.Err].
func (e ErrReader) Read([]byte) (n int, err error) {
	return 0, e.Err
}

// NewErrorAfterRandNReader returns an io.Reader that returns err after
// reading n random bytes from crypto/rand.Reader.
func NewErrorAfterRandNReader(n int, err error) io.Reader {
	return &errorAfterRandNReader{afterN: n, err: err}
}

var _ io.Reader = (*errorAfterRandNReader)(nil)

type errorAfterRandNReader struct {
	err    error
	afterN int
	count  int
	mu     sync.Mutex
}

func (r *errorAfterRandNReader) Read(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.count >= r.afterN {
		return 0, r.err
	}

	// There's some bytes to read
	allowed := r.afterN - r.count
	if allowed > len(p) {
		n, _ = crand.Read(p)
		r.count += n
		return n, nil
	}
	n, _ = crand.Read(p[:allowed])
	if n != allowed {
		panic(fmt.Sprintf("expected to read %d bytes, got %d", allowed, n))
	}
	r.count += n
	return n, r.err
}

var _ io.Reader = (*errorAfterBytesReader)(nil)

// NewErrorAfterBytesReader returns an io.Reader that returns err after
// p has been fully read. If err is nil, the reader will return io.EOF
// instead of err.
func NewErrorAfterBytesReader(p []byte, err error) io.Reader {
	r := &errorAfterBytesReader{err: err, buf: bytes.Buffer{}}
	_, _ = r.buf.Write(p)
	return r
}

type errorAfterBytesReader struct {
	err error
	buf bytes.Buffer
}

// Read implements io.Reader.
func (e *errorAfterBytesReader) Read(p []byte) (n int, err error) {
	n, err = e.buf.Read(p)
	if err != nil && e.err != nil {
		err = e.err
	}
	return n, err
}

// WriteErrorCloser supplements io.WriteCloser with an Error method, indicating
// to the io.WriteCloser that an upstream error has interrupted the writing
// operation. Note that clients should invoke only one of Close or Error.
type WriteErrorCloser interface {
	io.WriteCloser

	// Error indicates that an upstream error has interrupted the
	// writing operation.
	Error(err error)
}

type writeErrorCloser struct {
	fn func(error)
	io.WriteCloser
}

// Error implements WriteErrorCloser.Error.
func (w *writeErrorCloser) Error(err error) {
	if w.fn != nil {
		w.fn(err)
	}
}

// NewFuncWriteErrorCloser returns a new WriteErrorCloser that wraps w, and
// invokes non-nil fn when WriteErrorCloser.Error is called.
func NewFuncWriteErrorCloser(w io.WriteCloser, fn func(error)) WriteErrorCloser {
	return &writeErrorCloser{WriteCloser: w, fn: fn}
}
