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

// This code is lifted from github.com/dolmen-go/contextio.

package progress

import (
	"context"
	"io"
)

type progWriter struct {
	ctx context.Context
	w   io.Writer
}

type progCopier struct {
	progWriter
}

// NewProgWriter wraps an [io.Writer] to handle context cancellation.
//
// Context state is checked BEFORE every Write.
//
// The returned Writer also implements [io.ReaderFrom] to allow [io.Copy] to select
// the best strategy while still checking the context state before every chunk transfer.
func NewProgWriter(ctx context.Context, msg string, w io.Writer) io.Writer {
	if w, ok := w.(*progCopier); ok && ctx == w.ctx {
		return w
	}
	return &progCopier{progWriter{ctx: ctx, w: w}}
}

// Write implements [io.Writer], but with context awareness.
func (w *progWriter) Write(p []byte) (n int, err error) {
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
		return w.w.Write(p)
	}
}

func (w *progWriter) Close() error {
	// REVISIT: I'm not sure if we should always try
	// to close the underlying writer first, even if
	// the context is done? Or go straight to the
	// select ctx.Done?

	var closeErr error
	if wc, ok := w.w.(io.WriteCloser); ok {
		closeErr = wc.Close()
	}

	select {
	case <-w.ctx.Done():
		return w.ctx.Err()
	default:
	}

	return closeErr
}

type progReader struct {
	ctx context.Context
	r   io.Reader
}

// NewProgReader wraps an [io.Reader] to handle context cancellation.
//
// Context state is checked BEFORE every Read.
func NewProgReader(ctx context.Context, msg string, r io.Reader) io.Reader {
	if r, ok := r.(*progReader); ok && ctx == r.ctx {
		return r
	}
	return &progReader{ctx: ctx, r: r}
}

func (r *progReader) Read(p []byte) (n int, err error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.r.Read(p)
	}
}

// ReadFrom implements interface [io.ReaderFrom], but with context awareness.
//
// This should allow efficient copying allowing writer or reader to define the chunk size.
func (w *progCopier) ReadFrom(r io.Reader) (n int64, err error) {
	if _, ok := w.w.(io.ReaderFrom); ok {
		// Let the original Writer decide the chunk size.
		return io.Copy(w.progWriter.w, &progReader{ctx: w.ctx, r: r})
	}
	select {
	case <-w.ctx.Done():
		return 0, w.ctx.Err()
	default:
		// The original Writer is not a ReaderFrom.
		// Let the Reader decide the chunk size.
		return io.Copy(&w.progWriter, r)
	}
}
