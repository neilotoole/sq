// Package diffdoc provides core diff functionality, with a focus on streaming
// and concurrency.
//
// Reference:
//
// - https://en.wikipedia.org/wiki/Diff#Unified_format
// - https://www.gnu.org/software/diffutils/manual/html_node/Hunks.html
// - https://www.gnu.org/software/diffutils/manual/html_node/Sections.html
// - https://www.cloudbees.com/blog/git-diff-a-complete-comparison-tutorial-for-git
// - https://github.com/aymanbagabas/go-udiff
package diffdoc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/neilotoole/sq/libsq/core/bytez"
	"github.com/neilotoole/sq/libsq/core/colorz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"github.com/neilotoole/sq/libsq/core/langz"
	"github.com/neilotoole/sq/libsq/core/record"
)

var _ io.ReadCloser = (Doc)(nil)

// Doc is a diff document that implements [io.ReadCloser]. It is used to stream
// diff output.
type Doc interface {
	// Read provides access to the Doc's bytes. It blocks until the doc is sealed,
	// or returns a non-nil error. If the doc does not contain any diff hunks,
	// Read returns [io.EOF].
	Read(p []byte) (n int, err error)

	// Close closes the doc, releasing any resources held.
	Close() error

	// Err returns the error associated with the doc. On the happy path, Err
	// returns nil. If Err returns non-nil, a call to [Doc.Read] returns the same
	// error.
	Err() error

	// String returns the doc's title, with any colorization removed. It may be
	// empty. It exists mainly for logging and debugging.
	String() string
}

var (
	_ Doc       = (*UnifiedDoc)(nil)
	_ io.Writer = (*UnifiedDoc)(nil)
)

// NewUnifiedDoc returns a new [UnifiedDoc] with the given title. The title may
// be empty. The diff body should be written to via [UnifiedDoc.Write], and then
// the doc should be sealed via [UnifiedDoc.Seal].
func NewUnifiedDoc(cmdTitle Title) *UnifiedDoc {
	return &UnifiedDoc{
		title:   bytez.TerminateNewline(cmdTitle),
		sealed:  make(chan struct{}),
		bodyBuf: &bytes.Buffer{},
	}
}

var (
	_ Doc       = (*UnifiedDoc)(nil)
	_ io.Writer = (*UnifiedDoc)(nil)
)

// UnifiedDoc is a diff [Doc] that consists of a single unified diff body
// (although that body may contain multiple hunks). It exists as a bridge to
// legacy code that generates unified diff output as a single block of text.
//
// See also: [HunkDoc].
type UnifiedDoc struct {
	err     error
	rdr     io.Reader
	sealed  chan struct{}
	bodyBuf *bytes.Buffer
	title   Title
	rdrOnce sync.Once
	mu      sync.Mutex
}

// Close implements io.Closer.
func (d *UnifiedDoc) Close() error {
	d.bodyBuf.Reset()
	d.bodyBuf = nil
	return nil
}

// String returns the doc's title as a string, with any colorization removed.
// It may be empty.
func (d *UnifiedDoc) String() string {
	if d == nil || len(d.title) == 0 {
		return ""
	}
	return d.title.String()
}

// Write writes to the doc body. The bytes are returned (without additional
// processing) by [UnifiedDoc.Read], so any colorization etc. must occur before
// writing. When writing is complete, the doc must be sealed via
// [UnifiedDoc.Seal]. It is a programming error to invoke [UnifiedDoc.Write]
// after [UnifiedDoc.Seal] has been invoked.
func (d *UnifiedDoc) Write(p []byte) (n int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	n, err = d.bodyBuf.Write(p)
	return n, errz.Err(err)
}

// Seal seals the doc, indicating that it is complete. Until it is sealed, a
// call to [UnifiedDoc.Read] will block. On the happy path, arg err is nil. If
// err is non-nil, a call to [UnifiedDoc.Read] will return that error. Seal
// panics if called more than once.
func (d *UnifiedDoc) Seal(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	select {
	case <-d.sealed:
		panic("diff doc is already sealed")
	default:
	}

	d.err = err
	close(d.sealed)
}

// Read provides access to the doc's bytes. It blocks until the doc is sealed,
// or returns the non-nil error provided to [HunkDoc.Seal]. If the doc does not
// contain any diff hunks, Read returns [io.EOF].
func (d *UnifiedDoc) Read(p []byte) (n int, err error) {
	d.rdrOnce.Do(func() {
		<-d.sealed

		if d.err != nil {
			d.rdr = ioz.ErrReader{Err: d.err}
			return
		}

		if d.bodyBuf.Len() == 0 {
			d.rdr = ioz.EmptyReader{}
			return
		}

		if len(d.title) == 0 {
			d.rdr = d.bodyBuf
			return
		}

		d.rdr = io.MultiReader(bytes.NewReader(d.title), d.bodyBuf)
	})

	return d.rdr.Read(p)
}

// Err returns the error associated with the doc, as provided to
// [UnifiedDoc.Seal]. The same non-nil error is returned by a call to
// [UnifiedDoc.Read].
func (d *UnifiedDoc) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// Header is the byte sequence for a diff doc, as created by [Headerf],
// and passed to [NewHunkDoc].
type Header []byte

// String returns the header as a string. It may be empty. Colorization is
// stripped.
func (h Header) String() string {
	if len(h) == 0 {
		return ""
	}
	return string(colorz.Strip(h))
}

// Headerf formats a diff doc header suitable for use with [NewHunkDoc].
//
//	header := libdiff.Headerf(clrs, "@sakila_a.actor", "@sakila_b.actor")
//
// The returned header looks something like:
//
//	--- @sakila_a.actor
//	+++ @sakila_b.actor
//
// It is colorized according to [Colors.Header], and terminates with newline.
func Headerf(clrs *Colors, left, right string) Header {
	if clrs == nil || clrs.Header == nil || clrs.IsMonochrome() {
		return []byte(fmt.Sprintf("--- %s\n+++ %s\n", left, right))
	}

	line1 := clrs.Header.Sprintf("--- %s", left) + "\n"
	line2 := clrs.Header.Sprintf("+++ %s", right) + "\n"

	return append([]byte(line1), line2...)
}

// Title is the byte sequence for a diff command title, as created by [Titlef],
// and passed to [NewUnifiedDoc] or [NewHunkDoc].
type Title []byte

// String returns the title as a string. It may be empty. Colorization is
// stripped.
func (t Title) String() string {
	if len(t) == 0 {
		return ""
	}
	return string(colorz.Strip(t))
}

// Titlef formats a diff command title suitable for use with [NewHunkDoc] or
// [NewUnifiedDoc].
//
//	title := libdiff.Titlef(clrs, "sq diff --data %s %s", src1.Handle, src2.Handle)
//
// The title is colorized according to [Colors.CmdTitle] and terminates with
// newline.
func Titlef(clrs *Colors, format string, a ...any) []byte {
	title := fmt.Sprintf(format, a...)
	if title == "" {
		return []byte{}
	}

	if clrs != nil && clrs.CmdTitle != nil && !clrs.IsMonochrome() {
		title = clrs.CmdTitle.Sprint(title)
	}

	return bytez.TerminateNewline([]byte(title))
}

var _ Doc = (*HunkDoc)(nil)

// HunkDoc is a document that consists of a series of diff hunks. It implements
// [io.Reader], and is used to stream diff output. The hunks are added to the
// doc via [HunkDoc.NewHunk]. A call to [HunkDoc.Read] blocks until
// [HunkDoc.Seal] is invoked.
//
// This may seem overly elaborate, and the design can probably be simplified,
// but the idea is to stream individual diff hunks as they're generated, rather
// than buffering the entire diff in memory. This is important for large diffs
// where, in theory, each hunk could be gigabytes in size. An earlier
// implementation of this package had an [issue] where it consumed 20GB+ of
// memory to execute a complete diff of two reasonably small databases, so this
// isn't a purely theoretical concern.
//
// If the diff is only available as a block of unified diff text (as opposed to
// a sequence of hunks), instead use [UnifiedDoc].
//
// [issue]: https://github.com/neilotoole/sq/issues/353
type HunkDoc struct {
	err       error
	rdr       io.Reader
	sealed    chan struct{}
	closeErr  *error
	title     Title
	header    []byte
	hunks     []*Hunk
	rdrOnce   sync.Once
	closeOnce sync.Once
	mu        sync.Mutex
}

// Close implements io.Closer.
func (d *HunkDoc) Close() error {
	d.closeOnce.Do(func() {
		d.mu.Lock()
		var err error
		for i := range d.hunks {
			err = errz.Append(err, d.hunks[i].Close())
		}
		d.closeErr = &err
		d.hunks = nil
		d.mu.Unlock()
	})

	return *d.closeErr
}

// NewHunkDoc returns a new [HunkDoc] with the given title and header. The
// values should be previously colorized if desired. The title may be empty. The
// header can be generated with [Headerf]. If non-empty, both title and header
// should be terminated with a newline. The returned [HunkDoc] is not sealed;
// thus a call to [HunkDoc.Read] blocks until [HunkDoc.Seal] is invoked.
func NewHunkDoc(title Title, header []byte) *HunkDoc {
	return &HunkDoc{
		title:  title,
		header: header,
		sealed: make(chan struct{}),
	}
}

// String returns the doc's title as a string, with any colorization removed.
// It may be empty.
func (d *HunkDoc) String() string {
	if d == nil || len(d.title) == 0 {
		return ""
	}
	return d.title.String()
}

// Read provides access to the doc's bytes. It blocks until the doc is sealed,
// or returns the non-nil error provided to [HunkDoc.Seal]. If the doc does not
// contain any diff hunks, Read returns [io.EOF].
func (d *HunkDoc) Read(p []byte) (n int, err error) {
	d.rdrOnce.Do(func() {
		<-d.sealed

		if d.err != nil {
			d.rdr = ioz.ErrReader{Err: d.err}
			return
		}

		if len(d.hunks) == 0 {
			d.rdr = ioz.EmptyReader{}
			return
		}

		hunksMultiRdr := io.MultiReader(langz.MustTypedSlice[io.Reader](d.hunks...)...)
		p2 := make([]byte, len(p))
		n, err = hunksMultiRdr.Read(p2)
		switch {
		case n == 0 && errors.Is(err, io.EOF):
			d.rdr = ioz.EmptyReader{}
			return
		case n == 0 && err != nil:
			d.rdr = ioz.ErrReader{Err: err}
			return
		case n == 0 && err == nil:
			// Should be impossible because the hunks are buffers, and this
			// can't happen in our scenario?
			d.rdr = ioz.ErrReader{Err: errz.New("diff: hunks doc: unexpected zero read with nil error")}
		case err != nil:
			// n > 0, but we've hit an error.
			d.rdr = ioz.NewErrorAfterBytesReader(p2, err)
			return
		default:
			// Happy path: we've got some content in the hunks.
		}

		rdrs2 := make([]io.Reader, 0, 4)
		if len(d.title) > 0 {
			rdrs2 = append(rdrs2, bytes.NewReader(d.title))
		}
		if len(d.header) > 0 {
			rdrs2 = append(rdrs2, bytes.NewReader(d.header))
		}
		rdrs2 = append(rdrs2, bytes.NewReader(p2))
		rdrs2 = append(rdrs2, hunksMultiRdr)

		d.rdr = io.MultiReader(rdrs2...)
	})

	return d.rdr.Read(p)
}

// Seal seals the doc, indicating that it is complete. Until it is sealed, a
// call to [HunkDoc.Read] blocks. On the happy path, arg err is nil. If err is
// non-nil, a call to [HunkDoc.Read] returns an error. Seal panics if called
// more than once.
func (d *HunkDoc) Seal(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	select {
	case <-d.sealed:
		panic("diff doc is already sealed")
	default:
	}

	d.err = err
	close(d.sealed)
}

// Err returns the error associated with the doc, as provided to [HunkDoc.Seal].
// The same non-nil error is returned by a call to [HunkDoc.Read].
func (d *HunkDoc) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// NewHunk returns a new hunk, where offset is the nominal line number in the
// unified diff that this hunk would be part of. The returned hunk is not
// sealed, and any call to [Hunk.Read] will block until [Hunk.Seal] is invoked.
func (d *HunkDoc) NewHunk(offset int) (*Hunk, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.sealed:
		return nil, errz.New("diff doc is already sealed")
	default:
	}

	if d.closeErr != nil {
		return nil, errz.New("diff doc is already closed")
	}

	// TODO: new hunk should write out the previous hunk (if any) to
	// a HunkDoc.buf field, which probably should be
	// a https://pkg.go.dev/github.com/djherbis/buffer, using a memory/file
	// strategy.

	h := &Hunk{
		offset:  offset,
		sealed:  make(chan struct{}),
		bodyBuf: &bytes.Buffer{},
	}
	d.hunks = append(d.hunks, h)
	return h, nil
}

var (
	_ io.Writer     = (*Hunk)(nil)
	_ io.ReadCloser = (*Hunk)(nil)
)

// Hunk is a diff hunk, part of a [HunkDoc]. It implements [io.Writer] and
// [io.Reader]. The hunk is written to via [Hunk.Write], and then sealed via
// [Hunk.Seal]. Until sealed, [Hunk.Read] blocks.
type Hunk struct {
	err error

	rdr    io.Reader
	sealed chan struct{}
	// Consider using: https://pkg.go.dev/github.com/djherbis/buffer
	bodyBuf *bytes.Buffer

	header []byte

	offset  int
	rdrOnce sync.Once
	mu      sync.Mutex
}

// Offset returns the nominal offset of this hunk in its doc's body.
func (h *Hunk) Offset() int {
	return h.offset
}

// Close implements io.Closer.
func (h *Hunk) Close() error {
	h.header = nil
	h.bodyBuf = nil
	return nil
}

// Write writes to the hunk body. The hunk header ("@@ ... @@") should not be
// written to the body; instead it should be provided via [Hunk.Seal]. This
// facilitates stream processing of hunks, because the hunk header can't be
// calculated until after the hunk body is generated. When writing is complete,
// the hunk must be sealed via [Hunk.Seal], supplying the hunk header at that
// point.
//
// It is a programming error to invoke Write after [Hunk.Seal] or [Hunk.Close]
// has been invoked.
func (h *Hunk) Write(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	n, err = h.bodyBuf.Write(p)
	return n, errz.Err(err)
}

// Err returns any error associated with the hunk, as provided to [Hunk.Seal].
func (h *Hunk) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

// Seal seals the hunk, indicating that it is complete. The header arg is the
// hunk header ("@@ ... @@"). Until the hunk is sealed, a call to [Hunk.Read]
// blocks. On the happy path, arg err is nil. It is a runtime error to invoke
// Seal more than once.
func (h *Hunk) Seal(header []byte, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.header = header
	h.err = err
	close(h.sealed)
}

// Read blocks until the hunk is sealed. It returns the doc's bytes, or the
// non-nil error provided to [Hunk.Seal]. It is a programming error to invoke
// Read after [Hunk.Close] has been invoked.
func (h *Hunk) Read(p []byte) (n int, err error) {
	h.rdrOnce.Do(func() {
		<-h.sealed

		if h.err != nil {
			h.rdr = ioz.ErrReader{Err: h.err}
			return
		}

		h.rdr = io.MultiReader(bytes.NewReader(h.header), h.bodyBuf)
	})

	return h.rdr.Read(p)
}

type RecordDiffWriterFunc func(ctx context.Context, dest *Hunk, rm1, rm2 record.Meta, recPairs []record.Pair)
