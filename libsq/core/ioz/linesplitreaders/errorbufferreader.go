package linesplitreaders

import (
	"bytes"
	"io"
)

type errorBufferReader struct {
	buf *bytes.Buffer
	err error
}

func (e *errorBufferReader) Read(p []byte) (n int, err error) {
	if e.buf == nil {
		return 0, e.err
	}

	n, err = e.buf.Read(p)
	if err != nil {
		e.buf = nil
		err = e.err
	}

	return n, err
}

var _ io.Reader = (*errorBufferReader)(nil)

func newErrorBufferReader(b []byte, err error) *errorBufferReader {
	return &errorBufferReader{buf: bytes.NewBuffer(b), err: err}
}
