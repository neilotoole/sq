package diff

import (
	"fmt"
	"io"
	"strings"
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
	header string
	hunks  []*hunk
}

func newDiffDoc(title, left, right string) *diffDoc {
	header := fmt.Sprintf("%s\n--- %s\n+++ %s", title, left, right)
	return &diffDoc{header: header}
}

func (d *diffDoc) newHunk(row int) *hunk {
	// TODO: new hunk should write out the previous hunk (if any) to
	// a diffDoc.buf field, which probably should be
	// a https://pkg.go.dev/github.com/djherbis/buffer, using a memory/file
	// strategy.

	h := &hunk{row: row}
	d.hunks = append(d.hunks, h)
	return h
}

func (d *diffDoc) Reader() io.Reader {
	var rdrs []io.Reader
	for i := range d.hunks {
		rdrs = append(rdrs, d.hunks[i].Reader())
	}

	return io.MultiReader(rdrs...)
}

func (d *diffDoc) String() string {
	var sb strings.Builder
	_, _ = io.Copy(&sb, d.Reader())
	return sb.String()
}
