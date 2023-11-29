/*
Copyright 2018 Olivier Mengué

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This code is derived from github.com/dolmen-go/contextio.

package progress

import (
	"context"
	"errors"
	"io"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz/contextio"
)

// NewWriter returns an [io.Writer] that wraps w, is context-aware, and
// generates a progress bar as bytes are written to w. It is expected that ctx
// contains a *progress.Progress, as returned by progress.FromContext. If not,
// this function delegates to contextio.NewWriter: the returned writer will
// still be context-ware. See the contextio package for more details.
//
// Context state is checked BEFORE every Write.
//
// The returned [io.Writer] implements [io.ReaderFrom] to allow [io.Copy] to select
// the best strategy while still checking the context state before every chunk transfer.
//
// The returned [io.Writer] also implements [io.Closer], even if the underlying
// writer does not. This is necessary because we need a means of stopping the
// progress bar when writing is complete. If the underlying writer does
// implement [io.Closer], it will be closed when the returned writer is closed.
//
// If size is unknown, set to -1.
func NewWriter(ctx context.Context, msg string, size int64, w io.Writer) io.Writer {
	if w, ok := w.(*progCopier); ok && ctx == w.ctx {
		return w
	}

	pb := FromContext(ctx)
	if pb == nil {
		return contextio.NewWriter(ctx, w)
	}

	spinner := pb.NewByteCounter(msg, size)
	return &progCopier{progWriter{
		ctx:     ctx,
		w:       spinner.bar.ProxyWriter(w),
		spinner: spinner,
	}}
}

var _ io.WriteCloser = (*progWriter)(nil)

type progWriter struct {
	ctx     context.Context
	w       io.Writer
	spinner *Bar
}

// Write implements [io.Writer], but with context awareness.
func (w *progWriter) Write(p []byte) (n int, err error) {
	select {
	case <-w.ctx.Done():
		w.spinner.Stop()
		return 0, w.ctx.Err()
	default:
		n, err = w.w.Write(p)
		if err != nil {
			w.spinner.Stop()
		}
		return n, err
	}
}

// Close implements [io.WriteCloser], but with context awareness.
func (w *progWriter) Close() error {
	if w == nil {
		return nil
	}

	w.spinner.Stop()

	var closeErr error
	if c, ok := w.w.(io.Closer); ok {
		closeErr = errz.Err(c.Close())
	}

	select {
	case <-w.ctx.Done():
		ctxErr := w.ctx.Err()
		switch {
		case closeErr == nil,
			errz.IsErrContext(closeErr):
			return ctxErr
		default:
			return errors.Join(ctxErr, closeErr)
		}
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

	spinner := pb.NewByteCounter(msg, size)
	pr := &progReader{
		ctx:     ctx,
		r:       spinner.bar.ProxyReader(r),
		spinner: spinner,
	}
	return pr
}

var _ io.ReadCloser = (*progReader)(nil)

type progReader struct {
	ctx     context.Context
	r       io.Reader
	spinner *Bar
}

// Close implements [io.ReadCloser], but with context awareness.
func (r *progReader) Close() error {
	if r == nil {
		return nil
	}

	r.spinner.Stop()

	var closeErr error
	if c, ok := r.r.(io.ReadCloser); ok {
		closeErr = errz.Err(c.Close())
	}

	select {
	case <-r.ctx.Done():
		ctxErr := r.ctx.Err()
		switch {
		case closeErr == nil,
			errz.IsErrContext(closeErr):
			return ctxErr
		default:
			return errors.Join(ctxErr, closeErr)
		}
	default:
		return closeErr
	}
}

// Read implements [io.Reader], but with context awareness.
func (r *progReader) Read(p []byte) (n int, err error) {
	select {
	case <-r.ctx.Done():
		r.spinner.Stop()
		return 0, r.ctx.Err()
	default:
		n, err = r.r.Read(p)
		if err != nil {
			r.spinner.Stop()
		}
		return n, err
	}
}

var _ io.ReaderFrom = (*progCopier)(nil)

type progCopier struct {
	progWriter
}

// ReadFrom implements interface [io.ReaderFrom], but with context awareness.
//
// This should allow efficient copying allowing writer or reader to define the chunk size.
func (w *progCopier) ReadFrom(r io.Reader) (n int64, err error) {
	if _, ok := w.w.(io.ReaderFrom); ok {
		// Let the original Writer decide the chunk size.
		rdr := &progReader{
			ctx:     w.ctx,
			r:       w.spinner.bar.ProxyReader(r),
			spinner: w.spinner,
		}

		return io.Copy(w.progWriter.w, rdr)
	}
	select {
	case <-w.ctx.Done():
		w.spinner.Stop()
		return 0, w.ctx.Err()
	default:
		// The original Writer is not a ReaderFrom.
		// Let the Reader decide the chunk size.
		n, err = io.Copy(&w.progWriter, r)
		if err != nil {
			w.spinner.Stop()
		}
		return n, err
	}
}
