package progress

// Acknowledgement: The reader & writer implementations were originally
// adapted from github.com/dolmen-go/contextio.

import (
	"context"
	"io"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
)

// NewWriter returns a [progress.Writer] that wraps w, is context-aware, and
// generates a progress bar as bytes are written to w. It is expected that ctx
// contains a *progress.Progress, as returned by progress.FromContext. If not,
// this function delegates to contextio.NewWriter: the returned writer will
// still be context-aware. See the contextio package for more details.
//
// Context state is checked BEFORE every Write.
//
// The returned [progress.Writer] implements [io.ReaderFrom] to allow [io.Copy]
// to select the best strategy while still checking the context state before
// every chunk transfer.
//
// The returned [progress.Writer] also implements [io.Closer], even if the
// underlying writer does not. This is necessary because we need a means of
// stopping the progress bar when writing is complete. If the underlying writer
// does implement [io.Closer], it will be closed when the returned writer is
// closed.
//
// The caller is expected to close the returned writer, which results in the
// progress bar being removed. However, the progress bar can also be removed
// independently of closing the writer by invoking [Writer.Stop].
//
// If size is unknown, set to -1; this will result in an indeterminate progress
// spinner instead of a bar.
func NewWriter(ctx context.Context, msg string, size int64, w io.Writer) Writer {
	if w, ok := w.(*progCopier); ok && ctx == w.ctx {
		return w
	}

	pb := FromContext(ctx)
	if pb == nil {
		// No progress bar in context, so we delegate to contextio.
		return writerAdapter{contextio.NewWriter(ctx, w)}
	}

	b := pb.NewByteCounter(msg, size)
	return &progCopier{progWriter{
		ctx:     ctx,
		delayCh: b.delayCh,
		w:       w,
		b:       b,
	}}
}

var _ io.WriteCloser = (*progWriter)(nil)

type progWriter struct {
	ctx     context.Context
	w       io.Writer
	delayCh <-chan struct{}
	b       *Bar
}

// Write implements [io.Writer], but with context and progress interaction.
func (w *progWriter) Write(p []byte) (n int, err error) {
	select {
	case <-w.ctx.Done():
		w.b.Stop()
		return 0, w.ctx.Err()
	case <-w.delayCh:
		w.b.barInitOnce.Do(w.b.barInitFn)
	default:
	}

	n, err = w.w.Write(p)
	w.b.IncrBy(n)
	if err != nil {
		w.b.Stop()
	}
	return n, err
}

// Close implements [io.WriteCloser], but with context and
// progress interaction.
func (w *progWriter) Close() error {
	w.b.Stop()

	var closeErr error
	if c, ok := w.w.(io.Closer); ok {
		closeErr = errz.Err(c.Close())
	}

	select {
	case <-w.ctx.Done():
		return w.ctx.Err()

	default:
		return closeErr
	}
}

// NewReader returns an [io.Reader] that wraps r, is context-aware, and
// generates a progress bar as bytes are read from r. It is expected that ctx
// contains a *progress.Progress, as returned by progress.FromContext. If not,
// this function delegates to contextio.NewReader: the returned reader will
// still be context-ware. See the contextio package for more details.
//
// Context state is checked BEFORE every Read.
//
// The returned [io.Reader] also implements [io.Closer], even if the underlying
// reader does not. This is necessary because we need a means of stopping the
// progress bar when writing is complete. If the underlying reader does
// implement [io.Closer], it will be closed when the returned reader is closed.
func NewReader(ctx context.Context, msg string, size int64, r io.Reader) io.Reader {
	if r, ok := r.(*progReader); ok && ctx == r.ctx {
		return r
	}

	pb := FromContext(ctx)
	if pb == nil {
		return contextio.NewReader(ctx, r)
	}

	b := pb.NewByteCounter(msg, size)
	pr := &progReader{
		ctx:     ctx,
		delayCh: b.delayCh,
		r:       r,
		b:       b,
	}
	return pr
}

var _ io.ReadCloser = (*progReader)(nil)

type progReader struct {
	ctx     context.Context
	r       io.Reader
	delayCh <-chan struct{}
	b       *Bar
}

// Close implements [io.ReadCloser], but with context awareness.
func (r *progReader) Close() error {
	r.b.Stop()

	var closeErr error
	if c, ok := r.r.(io.ReadCloser); ok {
		closeErr = errz.Err(c.Close())
	}

	select {
	case <-r.ctx.Done():
		return r.ctx.Err()
	default:
		return closeErr
	}
}

// Read implements [io.Reader], but with context and progress interaction.
func (r *progReader) Read(p []byte) (n int, err error) {
	select {
	case <-r.ctx.Done():
		r.b.Stop()
		return 0, r.ctx.Err()
	case <-r.delayCh:
		r.b.barInitOnce.Do(r.b.barInitFn)
	default:
	}

	n, err = r.r.Read(p)
	r.b.IncrBy(n)
	if err != nil {
		r.b.Stop()
	}
	return n, err
}

var _ io.ReaderFrom = (*progCopier)(nil)

// Writer is an [io.WriteCloser] as returned by [NewWriter].
type Writer interface {
	io.WriteCloser

	// Stop stops and removes the progress bar. Typically this is accomplished
	// by invoking Writer.Close, but there are circumstances where it may
	// be desirable to stop the progress bar without closing the underlying
	// writer.
	Stop()
}

var _ Writer = (*writerAdapter)(nil)

// writerAdapter wraps an io.Writer to implement [progress.Writer].
// This is only used, by [NewWriter], when there is no progress bar
// in the context, and thus [NewWriter] delegates to contextio.NewWriter,
// but we still need to implement [progress.Writer].
type writerAdapter struct {
	io.Writer
}

// Close implements [io.WriteCloser]. If the underlying
// writer implements [io.Closer], it will be closed.
func (w writerAdapter) Close() error {
	if c, ok := w.Writer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Stop implements [Writer] and is no-op.
func (w writerAdapter) Stop() {
}

var _ Writer = (*progCopier)(nil)

type progCopier struct {
	progWriter
}

// Stop implements [progress.Writer].
func (w *progCopier) Stop() {
	w.b.Stop()
}

// ReadFrom implements [io.ReaderFrom], but with context and
// progress interaction.
func (w *progCopier) ReadFrom(r io.Reader) (n int64, err error) {
	if _, ok := w.w.(io.ReaderFrom); ok {
		// Let the original Writer decide the chunk size.
		rdr := &progReader{
			ctx:     w.ctx,
			delayCh: w.delayCh,
			r:       r,
			b:       w.b,
		}

		return io.Copy(w.progWriter.w, rdr)
	}
	select {
	case <-w.ctx.Done():
		w.b.Stop()
		return 0, w.ctx.Err()
	default:
		// The original Writer is not a ReaderFrom.
		// Let the Reader decide the chunk size.
		n, err = io.Copy(&w.progWriter, r)
		if err != nil {
			w.b.Stop()
		}
		return n, err
	}
}
