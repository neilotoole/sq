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

// buildDocHeader returns a diff header suitable for use with newHunkDoc. The
// returned header will look something like:
//
//	sq diff --data @diff/sakila_a.actor @diff/sakila_b.actor
//	--- @diff/sakila_a.actor
//	+++ @diff/sakila_b.actor
//
// It is colorized according to [output.Printing.DiffHeader].
func buildDocHeader(pr *output.Printing, title, left, right string) []byte {
	buf := &bytes.Buffer{}
	header := fmt.Sprintf("%s\n--- %s\n+++ %s\n", title, left, right)
	_, _ = colorz.NewPrinter(pr.DiffHeader).Block(buf, []byte(header))
	return buf.Bytes()
}

var _ io.Reader = (*hunkDoc)(nil)

// hunkDoc is a document that contains a series of diff hunks. It implements
// io.Reader, and is intended to be used to stream diff output. The hunks
// are added to the hunkDoc via newHunk. Any call to hunkDoc.Read will block
// until hunkDoc.Seal is invoked.
type hunkDoc struct {
	mu      sync.Mutex
	header  []byte
	sealed  chan struct{}
	hunks   []*hunk
	err     error
	rdrOnce sync.Once
	rdr     io.Reader
}

// newHunkDoc returns a new hunkDoc with the given docHeader. The header
// can be generated with buildDocHeader. It should look something like:
//
//	sq diff --data @diff/sakila_a.actor @diff/sakila_b.actor
//	--- @diff/sakila_a.actor
//	+++ @diff/sakila_b.actor
//
// The returned hunkDoc is not sealed, and any call to hunkDoc.Read will
// block until hunkDoc.Seal is invoked.
func newHunkDoc(docHeader []byte) *hunkDoc {
	return &hunkDoc{header: docHeader, sealed: make(chan struct{})}
}

// Read implements [io.Reader]. It blocks until the hunkDoc is sealed.
func (d *hunkDoc) Read(p []byte) (n int, err error) {
	d.rdrOnce.Do(func() {
		<-d.sealed

		if d.err != nil {
			d.rdr = ioz.ErrReader{Err: d.err}
			return
		}

		rdrs := make([]io.Reader, 0, len(d.hunks)+1)
		rdrs = append(rdrs, bytes.NewReader(d.header))
		for i := range d.hunks {
			rdrs = append(rdrs, d.hunks[i].Reader())
		}

		d.rdr = io.MultiReader(rdrs...)
	})

	return d.rdr.Read(p)
}

// Seal seals the hunkDoc, indicating that it is complete. Until it is sealed,
// any call to hunkDoc.Reader will block. On the happy path, arg err is nil. If
// err is non-nil, a call to hunkDoc.Reader will return an error. Seal will
// panic if called more than once.
func (d *hunkDoc) Seal(err error) {
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

func (d *hunkDoc) newHunk(row int) (*hunk, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.sealed:
		return nil, errz.New("diff doc is already sealed")
	default:
	}

	// TODO: new hunk should write out the previous hunk (if any) to
	// a hunkDoc.buf field, which probably should be
	// a https://pkg.go.dev/github.com/djherbis/buffer, using a memory/file
	// strategy.

	h := &hunk{row: row}
	d.hunks = append(d.hunks, h)
	return h, nil
}

// String returns d's header as a string.
func (d *hunkDoc) String() string {
	return string(d.header)
}

type hunk struct {
	header string

	// Consider using: https://pkg.go.dev/github.com/djherbis/buffer
	body string
	row  int
}

func (h *hunk) String() string {
	return h.header + "\n" + h.body
}

func (h *hunk) Reader() io.Reader {
	return strings.NewReader(h.String())
}
