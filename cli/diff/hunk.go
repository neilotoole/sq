package diff

import "strings"

type hunk struct {
	row    int
	header string
	body   string
}

func (h *hunk) String() string {
	return h.header + "\n" + h.body
}

type hunkAssembler struct {
	hunks []*hunk
}

func newHunkAssembler() *hunkAssembler {
	return &hunkAssembler{}
}

func (ha *hunkAssembler) newHunk(row int) *hunk {
	h := &hunk{row: row}
	ha.hunks = append(ha.hunks, h)
	return h
}

func (ha *hunkAssembler) String() string {
	var sb strings.Builder
	for i := range ha.hunks {
		sb.WriteString(ha.hunks[i].String())
	}

	return sb.String()
}
