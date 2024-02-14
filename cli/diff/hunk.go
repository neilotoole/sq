package diff

import (
	"io"
	"strings"
)

type hunk struct {
	row    int
	header string

	// Consider using: https://pkg.go.dev/github.com/djherbis/buffer
	body string
}

func (h *hunk) String() string {
	return h.header + "\n" + h.body
}

func (h *hunk) Reader() io.Reader {
	return strings.NewReader(h.String())
}

type hunkAssembler struct {
	header string
	hunks []*hunk
}

func newHunkAssembler() *hunkAssembler {
	return &hunkAssembler{}
}

func (ha *hunkAssembler) newHunk(row int) *hunk {
	// TODO: new hunk should write out the previous hunk (if any) to
	// a hunkAssembler.buf field, which probably should be
	// a https://pkg.go.dev/github.com/djherbis/buffer, using a memory/file
	// strategy.

	h := &hunk{row: row}
	ha.hunks = append(ha.hunks, h)
	return h
}

func (ha *hunkAssembler) Reader() io.Reader {
	var rdrs []io.Reader
	for i := range ha.hunks {
		rdrs = append(rdrs, ha.hunks[i].Reader())
	}

	return io.MultiReader(rdrs...)
}

func (ha *hunkAssembler) String() string {
	var sb strings.Builder
	_, _ = io.Copy(&sb, ha.Reader())
	return sb.String()
}
