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

// Package contextio provides io decorators that are context-aware.
package contextio

import (
	"context"
	"io"
)

var _ io.Writer = (*writer)(nil)

type writer struct {
	ctx context.Context
	w   io.Writer
}

var _ io.WriteCloser = (*writeCloser)(nil)

type writeCloser struct {
	writer
}

var _ io.ReaderFrom = (*copier)(nil)

type copier struct {
	writer
}

var _ io.WriteCloser = (*copyCloser)(nil)

type copyCloser struct {
	writeCloser
}

// NewWriter wraps an io.Writer to handle context cancellation.
//
// Context state is checked BEFORE every Write.
//
// The returned Writer also implements io.ReaderFrom to allow io.Copy to select
// the best strategy while still checking the context state before every chunk transfer.
//
// If w implements io.WriteCloser, the returned Writer will
// also implement io.WriteCloser.
func NewWriter(ctx context.Context, w io.Writer) io.Writer {
	if w, ok := w.(*copier); ok && ctx == w.ctx {
		return w
	}

	if w, ok := w.(*copyCloser); ok && ctx == w.ctx {
		return w
	}

	wr := writer{ctx: ctx, w: w}
	if _, ok := w.(io.Closer); ok {
		return &copyCloser{writeCloser: writeCloser{writer: wr}}
	}

	return &copier{writer: wr}
}

// Write implements io.Writer, but with context awareness.
func (w *writer) Write(p []byte) (n int, err error) {
	select {
	case <-w.ctx.Done():
		return 0, cause(w.ctx, nil)
	default:
		n, err = w.w.Write(p)
		err = cause(w.ctx, err)
		return n, err
	}
}

// Close implements io.Closer, but with context awareness.
func (w *writeCloser) Close() error {
	var closeErr error
	if c, ok := w.w.(io.Closer); ok {
		closeErr = c.Close()
	}

	select {
	case <-w.ctx.Done():
		return cause(w.ctx, nil)
	default:
		return closeErr
	}
}

var _ io.Reader = (*reader)(nil)

type reader struct {
	ctx context.Context
	r   io.Reader
}

// NewReader wraps an io.Reader to handle context cancellation.
//
// Context state is checked BEFORE every Read.
//
// If r implements io.ReadCloser, the returned reader will
// also implement io.ReadCloser.
func NewReader(ctx context.Context, r io.Reader) io.Reader {
	if r, ok := r.(*reader); ok && ctx == r.ctx {
		return r
	}

	if r, ok := r.(*readCloser); ok && ctx == r.ctx {
		return r
	}

	rdr := reader{ctx: ctx, r: r}
	if _, ok := r.(io.ReadCloser); ok {
		return &readCloser{rdr}
	}

	return &rdr
}

// Read implements io.Reader, but with context awareness.
func (r *reader) Read(p []byte) (n int, err error) {
	select {
	case <-r.ctx.Done():
		return 0, cause(r.ctx, nil)
	default:
		n, err = r.r.Read(p)
		err = cause(r.ctx, err)
		return n, err
	}
}

var _ io.ReadCloser = (*readCloser)(nil)

type readCloser struct {
	reader
}

// Close implements io.Closer, but with context awareness.
func (rc *readCloser) Close() error {
	var closeErr error
	if c, ok := rc.r.(io.Closer); ok {
		closeErr = c.Close()
	}

	select {
	case <-rc.ctx.Done():
		return cause(rc.ctx, nil)

	default:
		return closeErr
	}
}

// ReadFrom implements interface io.ReaderFrom, but with context awareness.
//
// This should allow efficient copying allowing writer or reader to define the chunk size.
func (w *copier) ReadFrom(r io.Reader) (n int64, err error) {
	if _, ok := w.w.(io.ReaderFrom); ok {
		// Let the original Writer decide the chunk size.
		return io.Copy(w.w, &reader{ctx: w.ctx, r: r})
	}
	select {
	case <-w.ctx.Done():
		return 0, cause(w.ctx, nil)
	default:
		// The original Writer is not a ReaderFrom.
		// Let the Reader decide the chunk size.
		n, err = io.Copy(&w.writer, r)
		err = cause(w.ctx, err)
		return n, err
	}
}

// NewCloser wraps an io.Reader to handle context cancellation.
//
// The underlying io.Closer is closed even if the context is done.
func NewCloser(ctx context.Context, c io.Closer) io.Closer {
	return &closer{ctx: ctx, c: c}
}

type closer struct {
	ctx context.Context
	c   io.Closer
}

func (c *closer) Close() error {
	closeErr := c.c.Close()

	select {
	case <-c.ctx.Done():
		return cause(c.ctx, nil)

	default:
		return closeErr
	}
}

func cause(ctx context.Context, err error) error {
	if err == nil {
		return context.Cause(ctx)
	}

	// err is non-nil
	if ctx.Err() != err { //nolint:errorlint
		// err is not the context error, so err takes precedence.
		return err
	}

	// err is the context error. Return the cause.
	return context.Cause(ctx)
}
