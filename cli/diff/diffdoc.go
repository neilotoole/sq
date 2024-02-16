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

var _ io.Reader = (*diffDoc)(nil)

func buildDocHeader(pr *output.Printing, title, left, right string) []byte {
	buf := &bytes.Buffer{}
	header := fmt.Sprintf("%s\n--- %s\n+++ %s\n", title, left, right)
	_, _ = colorz.NewPrinter(pr.DiffHeader).Block(buf, []byte(header))
	return buf.Bytes()
}

type diffDoc struct {
	mu      sync.Mutex
	header  []byte
	sealed  chan struct{}
	hunks   []*hunk
	err     error
	rdrOnce sync.Once
	rdr     io.Reader
}

func newDiffDoc(docHeader []byte) *diffDoc {
	return &diffDoc{header: docHeader, sealed: make(chan struct{})}
}

// Read implements [io.Reader]. It blocks until the diffDoc is sealed.
func (d *diffDoc) Read(p []byte) (n int, err error) {
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

// Seal seals the diffDoc, indicating that it is complete. Until it is sealed,
// any call to diffDoc.Reader will block. On the happy path, arg err is nil. If
// err is non-nil, a call to diffDoc.Reader will return an error. Seal will
// panic if called more than once.
func (d *diffDoc) Seal(err error) {
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

func (d *diffDoc) newHunk(row int) (*hunk, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	select {
	case <-d.sealed:
		return nil, errz.New("diff doc is already sealed")
	default:
	}

	// TODO: new hunk should write out the previous hunk (if any) to
	// a diffDoc.buf field, which probably should be
	// a https://pkg.go.dev/github.com/djherbis/buffer, using a memory/file
	// strategy.

	h := &hunk{row: row}
	d.hunks = append(d.hunks, h)
	return h, nil
}

// String returns d's header as a string.
func (d *diffDoc) String() string {
	return string(d.header)
}
