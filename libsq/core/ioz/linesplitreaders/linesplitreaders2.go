// Package linesplitreaders provides a type that returns an io.Reader for
// each line in the source io.Reader.
package linesplitreaders

import (
	"bytes"
	"io"
)

/*

https://stackoverflow.com/questions/37530451/golang-bufio-read-multiline-until-crlf-r-n-delimiter/37531472
*/

type Splitter struct {
	src       io.Reader
	buf       *bytes.Buffer
	srcErr    error
	activeRdr *reader
	trailingR bool
}

func New(src io.Reader) *Splitter {
	return &Splitter{src: src, buf: &bytes.Buffer{}}
}

// Next returns true if there is another reader available via Splitter.Reader.
func (s *Splitter) Next() bool {
	return s.srcErr == nil
}

// Reader returns the next reader, or nil.
func (s *Splitter) Reader() io.Reader {
	if !s.Next() {
		return nil
	}
	if s.activeRdr != nil && s.activeRdr.err == nil {
		panic("active reader not done")
	}
	r := &reader{sc: s}
	s.activeRdr = r
	return r
}

var _ io.Reader = &reader{}

type reader struct {
	sc  *Splitter
	err error
}

func (r *reader) Read(p []byte) (n int, err error) {
	if r.sc.srcErr != nil {
		return 0, r.sc.srcErr
	}

	if r.err != nil {
		return 0, r.err
	}

	if len(p) == 0 {
		return 0, r.err
	}

	if r.sc.buf.Len() > 0 {
		return r.handleBuf(p)
	}

	n, err = r.sc.src.Read(p)
	if err != nil {
		return r.handleReadError(p, n, err)
	}

	if n == 0 { // FIXME: revisit
		return 0, nil
	}

	// n >= 1
	if r.sc.trailingR {
		return r.handleTrailingR(p, n)
	}

	lfi := bytes.IndexByte(p[:n], '\n')

	// Options
	// 1. Leading newline
	// 2. No newline
	// 3. Trailing newline
	// 4. Middle newline

	switch {
	case lfi == 0: // Leading newline
		return r.handleLeadingLF(p, n)

	case lfi < 0: // Didn't find a newline
		return r.handleNoLF(p, n)

	case lfi == n-1: // trailing newline
		return r.handleTrailingLF(p, n)

	default:
		return r.handleMiddleLF(p, n, lfi)
	}
}

func (r *reader) handleTrailingR(p []byte, n int) (int, error) {
	// n > 0
	r.sc.trailingR = false
	if p[0] == '\n' {
		r.sc.buf.Write(p[1:n])
		r.markReaderEOF()
		return 0, io.EOF
	}

	r.sc.buf.Write(p[:n])
	p[0] = '\r'
	return 1, nil
}

func (r *reader) handleReadError(p []byte, n int, err error) (int, error) {
	if n == 0 {
		r.err = err
		r.sc.srcErr = err
		r.sc.activeRdr = nil

		if r.sc.trailingR {
			r.sc.trailingR = false
			p[0] = '\r'
			return 1, err
		}

		return 0, err
	}

	if n == 1 {
		if p[0] == '\n' {
			r.sc.trailingR = false
			r.markReaderEOF()
			r.sc.srcErr = err
			return 0, io.EOF
		}
		//
		//return n, err
	}

	ebr := newErrorBufferReader(p[:n], err)
	r.sc.src = ebr
	return 0, nil
}

func (r *reader) handleBuf(p []byte) (n int, err error) {
	if r.sc.buf.Len() == 0 {
		panic("buf is empty")
	}

	data := r.sc.buf.Bytes()
	if len(data) > len(p) {
		data = data[:len(p)]
	}

	if r.sc.trailingR {
		r.sc.trailingR = false
		if data[0] == '\n' {
			r.sc.buf.ReadByte()
			r.markReaderEOF()
			return 0, io.EOF
		}

		p[0] = '\r'
		return 1, nil
	}

	lfi := bytes.IndexByte(data, '\n')
	switch {
	case lfi < 0:
		if data[len(data)-1] == '\r' {
			p = p[:len(data)-1]
			n, err = r.sc.buf.Read(p)
			r.sc.trailingR = true
			r.sc.buf.ReadByte() // discard the CR
			return n, err
		}

		n, err = r.sc.buf.Read(p)
		return n, err

	case lfi == 0:
		_, _ = r.sc.buf.ReadByte()
		r.markReaderEOF()
		return 0, io.EOF

	case lfi == 1:
		if data[0] == '\r' {
			_, _ = r.sc.buf.ReadByte() // discard the CR
			_, _ = r.sc.buf.ReadByte() // discard the LF
			r.markReaderEOF()
			return 0, io.EOF
		}

		p[0], _ = r.sc.buf.ReadByte()
		_, _ = r.sc.buf.ReadByte() // discard the LF
		r.markReaderEOF()
		return 1, io.EOF

	default:
		// lfi > 1, continues below
		if data[lfi-1] == '\r' {
			p = p[:lfi-1]
			n, err = r.sc.buf.Read(p)
			_, _ = r.sc.buf.ReadByte() // discard the CR
			_, _ = r.sc.buf.ReadByte() // discard the LF
			r.markReaderEOF()
			return n, err
		}

		p = p[:lfi]
		n, err = r.sc.buf.Read(p)
		r.sc.buf.ReadByte() // discard the LF
		r.markReaderEOF()
		return n, err
	}
}

func (r *reader) markReaderEOF() {
	r.sc.activeRdr = nil
	r.err = io.EOF
}

func (r *reader) handleLeadingLF(p []byte, srcN int) (n int, err error) {
	if r.sc.trailingR {
		r.sc.trailingR = false
	}

	r.markReaderEOF()
	r.sc.buf.Write(p[1:srcN])
	return 0, io.EOF
}

func (r *reader) handleNoLF(p []byte, n int) (int, error) {
	if p[n-1] == '\r' {
		n--
		r.sc.trailingR = true
	}

	return n, nil
}

func (r *reader) handleTrailingLF(p []byte, srcN int) (n int, err error) {
	if srcN > 1 && p[srcN-2] == '\r' {
		srcN--
	}

	r.markReaderEOF()
	return srcN - 1, io.EOF
}

func (r *reader) handleMiddleLF(p []byte, n, lfi int) (int, error) {
	_, _ = r.sc.buf.Write(p[lfi+1 : n])
	r.markReaderEOF()

	if n > 1 && p[lfi-1] == '\r' {
		return lfi - 1, io.EOF
	}

	return lfi, io.EOF
}
