package diff

import (
	"bytes"
	"fmt"
	"github.com/neilotoole/sq/cli/output"
	"github.com/neilotoole/sq/cli/output/colorz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/ioz"
	"io"
	"sync"
)

// buildDocHeader returns a diff header suitable for use with newHunkDoc. The
// returned header will look something like:
//
//	--- @sakila_a.actor
//	+++ @sakila_b.actor
//
// It is colorized according to [output.Printing.DiffHeader].
func buildDocHeader(pr *output.Printing, left, right string) []byte {
	buf := &bytes.Buffer{}
	header := fmt.Sprintf("--- %s\n+++ %s\n", left, right)
	_, _ = colorz.NewPrinter(pr.DiffHeader).Block(buf, []byte(header))
	return buf.Bytes()
}

var _ io.Reader = (*hunkDoc)(nil)

// hunkDoc is a document that contains a series of diff hunks. It implements
// io.Reader, and is used to stream diff output. The hunks are added to the
// hunkDoc via NewHunk. Any call to hunkDoc.Read will block until hunkDoc.Seal
// is invoked.
//
// This may seem overly elaborate, and the design can probably be simplified,
// but the idea is to stream diff content as it's generated, rather than
// buffering the entire diff in memory. This is important for large diffs,
// which, in theory could be gigabytes in size. An earlier implementation of
// this package used 20GB+ of memory to execute a complete diff of two
// reasonably small databases.
type hunkDoc struct {
	mu      sync.Mutex
	title   string
	header  []byte
	sealed  chan struct{}
	hunks   []*hunk
	err     error
	rdrOnce sync.Once
	rdr     io.Reader
}

// newHunkDoc returns a new hunkDoc with the given title and header. Note that
// the title is not included in the output returned by hunkDoc.Read. This is
// because the title is only shown if the command output includes multiple docs.
// The header can be generated with buildDocHeader. The returned hunkDoc is not
// sealed; thus a call to hunkDoc.Read blocks until hunkDoc.Seal is invoked.
func newHunkDoc(title string, docHeader []byte) *hunkDoc {
	return &hunkDoc{
		title:  title,
		header: docHeader,
		sealed: make(chan struct{}),
	}
}

// Title returns the doc's title. It is in plaintext, without colorization.
func (hd *hunkDoc) Title() string {
	return hd.title
}

// Read implements [io.Reader]. It blocks until the hunkDoc is sealed.
func (hd *hunkDoc) Read(p []byte) (n int, err error) {
	hd.rdrOnce.Do(func() {
		<-hd.sealed

		if hd.err != nil {
			hd.rdr = ioz.ErrReader{Err: hd.err}
			return
		}

		rdrs := make([]io.Reader, 0, len(hd.hunks)+1)
		rdrs = append(rdrs, bytes.NewReader(hd.header))
		for i := range hd.hunks {
			rdrs = append(rdrs, hd.hunks[i])
		}

		hd.rdr = io.MultiReader(rdrs...)
	})

	return hd.rdr.Read(p)
}

// Seal seals the hunkDoc, indicating that it is complete. Until it is sealed,
// any call to hunkDoc.Read will block. On the happy path, arg err is nil. If
// err is non-nil, a call to hunkDoc.Reader will return an error. Seal will
// panic if called more than once.
func (hd *hunkDoc) Seal(err error) {
	hd.mu.Lock()
	defer hd.mu.Unlock()
	select {
	case <-hd.sealed:
		panic("diff hunk doc is already sealed")
	default:
	}

	hd.err = err
	close(hd.sealed)
}

// Err returns the error associated with the doc, as provided to Seal. On the
// happy path, Err returns nil.
func (hd *hunkDoc) Err() error {
	hd.mu.Lock()
	defer hd.mu.Unlock()
	return hd.err
}

// NewHunk returns a new hunk, where offset is the nominal line number in the
// unified diff that this hunk would be part of. The returned hunk is not
// sealed, and any call to hunk.Read will block until hunk.Seal is invoked.
func (hd *hunkDoc) NewHunk(offset int) (*hunk, error) {
	hd.mu.Lock()
	defer hd.mu.Unlock()

	select {
	case <-hd.sealed:
		return nil, errz.New("diff hunk doc is already sealed")
	default:
	}

	// TODO: new hunk should write out the previous hunk (if any) to
	// a hunkDoc.buf field, which probably should be
	// a https://pkg.go.dev/github.com/djherbis/buffer, using a memory/file
	// strategy.

	h := &hunk{
		offset:  offset,
		sealed:  make(chan struct{}),
		bodyBuf: &bytes.Buffer{},
	}
	hd.hunks = append(hd.hunks, h)
	return h, nil
}

// String returns the doc's header as a string.
func (hd *hunkDoc) String() string {
	return string(hd.header)
}

var _ io.Writer = (*hunk)(nil)
var _ io.Reader = (*hunk)(nil)

// hunk is a diff hunk. It implements io.Writer and io.Reader. The hunk is
// written to via Write, and then sealed via Seal. Once sealed, the hunk can
// be read via Read. Any call to hunk.Read will block until hunk.Seal is
// invoked.
type hunk struct {
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

// Write writes to the hunk body. When writing is completed, the hunk must be
// sealed via Seal. It is a programming error to invoke Write after Seal has
// been invoked.
func (h *hunk) Write(p []byte) (n int, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	n, err = h.bodyBuf.Write(p)
	return n, errz.Err(err)
}

// Err returns the error associated with the hunk, as provided to hunk.Seal.
// On the happy path, Err returns nil.
func (h *hunk) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

// Seal seals the hunk, indicating that it is complete. Until it is sealed, any
// call to hunk.Reader will block. On the happy path, arg err is nil. If an
func (h *hunk) Seal(header []byte, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.header = header
	h.err = err
	close(h.sealed)
}

// Read implements [io.Reader]. It blocks until the hunk is sealed.
func (h *hunk) Read(p []byte) (n int, err error) {
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
