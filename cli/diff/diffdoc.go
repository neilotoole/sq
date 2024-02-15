package diff

import (
	"context"
	"fmt"
	"github.com/neilotoole/sq/libsq/core/errz"
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

type diffDoc struct {
	mu     sync.Mutex
	header string
	sealed chan struct{}
	hunks  []*hunk
	err    error
}

func newDiffDoc(title, left, right string) *diffDoc {
	header := fmt.Sprintf("%s\n--- %s\n+++ %s", title, left, right)
	return &diffDoc{header: header, sealed: make(chan struct{})}
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

// Reader returns an io.Reader for the diffDoc. The method blocks
// until the diffDoc.Seal is invoked.
func (d *diffDoc) Reader(ctx context.Context) (io.Reader, error) {
	select {
	case <-ctx.Done():
		return nil, errz.Err(ctx.Err())
	case <-d.sealed:
		// continue
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.err != nil {
		return nil, d.err
	}

	rdrs := make([]io.Reader, 0, len(d.hunks)+1)
	//rdrs = append(rdrs, strings.NewReader(d.header))
	for i := range d.hunks {
		rdrs = append(rdrs, d.hunks[i].Reader())
	}

	return io.MultiReader(rdrs...), nil
}

// String returns the diffDoc's header.
func (d *diffDoc) String() string {
	return d.header
}
