package diff

import (
	"bytes"
	"fmt"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/colorz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"io"
	"strings"
	"sync"
)

var _ io.ReadCloser = (Doc)(nil)

// Doc is a diff document that implements [io.ReadCloser]. It is used to stream
// diff output.
type Doc interface {
	// Read provides access to the Doc's bytes. It blocks until the doc is sealed,
	// or returns a non-nil error.
	Read(p []byte) (n int, err error)

	// Close closes the doc, disposing of any resources held.
	Close() error

	// Title returns the doc's title, which may be empty. If non-empty, the title
	// is returned
	Title() string

	// Err returns the error associated with the doc. On the happy path, Err
	// returns nil.
	Err() error
}

var _ Doc = (*UnifiedDoc)(nil)
var _ io.Writer = (*UnifiedDoc)(nil)

func NewUnifiedDoc(title string) *UnifiedDoc {
	return &UnifiedDoc{
		title:   title,
		sealed:  make(chan struct{}),
		bodyBuf: &bytes.Buffer{},
	}
}

type UnifiedDoc struct {
	mu      sync.Mutex
	title   string
	sealed  chan struct{}
	err     error
	rdrOnce sync.Once
	rdr     io.Reader
	bodyBuf *bytes.Buffer
}

// Close implements io.Closer.
func (d *UnifiedDoc) Close() error {
	d.bodyBuf = nil
	return nil
}

// Title returns the doc's title, which may be empty.
func (d *UnifiedDoc) Title() string {
	return d.title
}

// Write writes to the doc body. The bytes are returned without processing by
// [Read], so any colorization etc. must occur before writing. When writing is
// completed, the doc must be sealed via [Seal]. It is a programming error to
// invoke Write after Seal has been invoked.
func (d *UnifiedDoc) Write(p []byte) (n int, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	n, err = d.bodyBuf.Write(p)
	return n, errz.Err(err)
}

// Seal seals the doc, indicating that it is complete. Until it is sealed, a
// call to [UnifiedDoc.Read] will block. On the happy path, arg err is nil. If
// err is non-nil, a call to [UnifiedDoc.Read] will return an error. Seal panics
// if called more than once.
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

// Read blocks until the doc is sealed. It returns the doc's bytes, or the
// non-nil error provided to [UnifiedDoc.Seal].
func (d *UnifiedDoc) Read(p []byte) (n int, err error) {
	d.rdrOnce.Do(func() {
		<-d.sealed

		if d.err != nil {
			d.rdr = ioz.ErrReader{Err: d.err}
			return
		}

		if d.title == "" {
			d.rdr = d.bodyBuf
			return
		}

		d.rdr = io.MultiReader(strings.NewReader(d.title+"\n"), d.bodyBuf)
	})

	return d.rdr.Read(p)
}

// Err returns any error associated with the doc, as provided to
// [UnifiedDoc.Seal].
func (d *UnifiedDoc) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// NewDocHeader returns a diff header suitable for use with NewHunkDoc. The
// returned header looks something like:
//
//	--- @sakila_a.actor
//	+++ @sakila_b.actor
//
// It is colorized according to [output.Printing.DiffHeader].
func NewDocHeader(pr *output.Printing, left, right string) []byte {
	buf := &bytes.Buffer{}
	header := fmt.Sprintf("--- %s\n+++ %s\n", left, right)
	_, _ = colorz.NewPrinter(pr.DiffHeader).Block(buf, []byte(header))
	return buf.Bytes()
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
	mu        sync.Mutex
	title     string
	header    []byte
	sealed    chan struct{}
	hunks     []*Hunk
	err       error
	rdrOnce   sync.Once
	rdr       io.Reader
	closeOnce sync.Once
	closeErr  *error
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

//
//func (hd *HunkDoc) Size() (size int64, err error) {
//	hd.mu.Lock()
//	defer hd.mu.Unlock()
//
//	if hd.err != nil {
//		return 0, hd.err
//	}
//
//	<-hd.sealed
//
//	var n int64
//	for i := range hd.hunks {
//		if n, err = hd.hunks[i].Size(); err != nil {
//			return 0, err
//		}
//		size += n
//	}
//
//	return size, nil
//}

// NewHunkDoc returns a new HunkDoc with the given title and header. The title
// may be empty. The header can be generated with NewDocHeader. The returned
// HunkDoc is not sealed; thus a call to hunkDoc.Read blocks until HunkDoc.Seal
// is invoked.
func NewHunkDoc(title string, header []byte) *HunkDoc {
	return &HunkDoc{
		title:  title,
		header: header,
		sealed: make(chan struct{}),
	}
}

// Title returns the doc's title, which may be empty.
func (d *HunkDoc) Title() string {
	return d.title
}

// Read blocks until the doc is sealed. It returns the doc's bytes, or the
// non-nil error provided to [HunkDoc.Seal].
func (d *HunkDoc) Read(p []byte) (n int, err error) {
	d.rdrOnce.Do(func() {
		<-d.sealed

		if d.err != nil {
			d.rdr = ioz.ErrReader{Err: d.err}
			return
		}

		rdrs := make([]io.Reader, 0, len(d.hunks)+2)
		if len(d.title) > 0 {
			rdrs = append(rdrs, strings.NewReader(d.title+"\n"))
		}
		rdrs = append(rdrs, bytes.NewReader(d.header))
		for i := range d.hunks {
			rdrs = append(rdrs, d.hunks[i])
		}

		d.rdr = io.MultiReader(rdrs...)
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
func (d *HunkDoc) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// NewHunk returns a new hunk, where offset is the nominal line number in the
// unified diff that this hunk would be part of. The returned hunk is not
// sealed, and any call to hunk.Read will block until hunk.Seal is invoked.
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

var _ io.Writer = (*Hunk)(nil)
var _ io.ReadCloser = (*Hunk)(nil)

// Hunk is a diff hunk. It implements io.Writer and io.Reader. The hunk is
// written to via Write, and then sealed via Seal. Once sealed, the hunk can
// be read via Read. Any call to hunk.Read will block until hunk.Seal is
// invoked.
type Hunk struct {
	mu     sync.Mutex
	sealed chan struct{}
	err    error

	offset int
	header []byte
	// Consider using: https://pkg.go.dev/github.com/djherbis/buffer
	bodyBuf *bytes.Buffer

	rdr     io.Reader
	rdrOnce sync.Once
}

// Close implements io.Closer.
func (h *Hunk) Close() error {
	h.header = nil
	h.bodyBuf = nil
	return nil
}

// Write writes to the hunk body. When writing is completed, the hunk must be
// sealed via Seal. It is a programming error to invoke Write after [Hunk.Seal]
// or [Hunk.Close] has been invoked.
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

// Seal seals the hunk, indicating that it is complete. Until it is sealed, a
// call to [Hunk.Read] blocks. On the happy path, arg err is nil. It is a
// runtime error to call Seal more than once.
func (h *Hunk) Seal(header []byte, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.header = header
	h.err = err
	close(h.sealed)
}

// Read blocks until the hunk is sealed. It returns the doc's bytes, or the
// non-nil error provided to [Hunk.Seal]. It is a programming error to call Read
// after [Hunk.Close] has been invoked.
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

//// Size returns the size of the hunk body (as returned by Read) in bytes. It
//// blocks until the hunk is sealed, or returns the hunk's non-nil error.
//func (h *Hunk) Size() (int64, error) {
//	h.mu.Lock()
//	if h.err != nil {
//		defer h.mu.Unlock()
//		return 0, h.err
//	}
//	h.mu.Unlock()
//	<-h.sealed
//	h.mu.Lock()
//	defer h.mu.Unlock()
//	return int64(h.bodyBuf.Len()), h.err
//}
