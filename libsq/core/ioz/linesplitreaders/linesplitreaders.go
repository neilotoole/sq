// Package linesplitreaders provides a Splitter that returns an io.Reader for
// each line in the source io.Reader. The end goal is similar to bufio.Scanner,
// but Splitter can process arbitrarily long lines, while bufio.Scanner is
// limited to the maximum size of the buffer (defaulting to bufio.MaxScanTokenSize).
package linesplitreaders

import (
	"bytes"
	"io"
)

/*

https://stackoverflow.com/questions/37530451/golang-bufio-read-multiline-until-crlf-r-n-delimiter/37531472
*/

// Splitter provides a mechanism to return an io.Reader for each line in the
// source io.Reader provided to New. Use Splitter.HasMore and Splitter.Reader to
// iterate.
type Splitter struct {
	src        io.Reader
	buf        *bytes.Buffer
	srcErr     error
	activeRdr  *reader
	trailingCR bool
}

// New returns a new Splitter.
func New(src io.Reader) *Splitter {
	return &Splitter{src: src, buf: &bytes.Buffer{}}
}

// HasMore returns true if there is another reader available via Splitter.Reader,
// or if there's an active reader that has not been fully consumed.
// HasMore returns true at least once, even if the source io.Reader returns zero
// bytes.
func (s *Splitter) HasMore() bool {
	return s.srcErr == nil
}

// Reader returns the reader for the next line, or nil if there are no more
// readers available. Reader will panic if the previous reader returned by this
// method has not been fully consumed.
func (s *Splitter) Reader() io.Reader {
	if !s.HasMore() {
		return nil
	}
	if s.activeRdr != nil && s.activeRdr.err == nil {
		panic("active reader not fully consumed")
	}
	r := &reader{sp: s}
	s.activeRdr = r
	return r
}

var _ io.Reader = &reader{}

type reader struct {
	sp *Splitter

	// err is stored so that subsequent calls to Read return the same error.
	err error
}

// Read implements io.Reader.
func (r *reader) Read(p []byte) (n int, err error) {
	if r.sp.srcErr != nil {
		return 0, r.sp.srcErr
	}

	if r.err != nil {
		return 0, r.err
	}

	if len(p) == 0 {
		return 0, r.err
	}

	if r.sp.buf.Len() > 0 {
		return r.handleBuf(p)
	}

	n, err = r.sp.src.Read(p)
	if err != nil {
		return r.handleReadError(p, n, err)
	}

	if n == 0 {
		return 0, nil
	}

	// n >= 1
	if r.sp.trailingCR {
		return r.handleTrailingCR(p, n)
	}

	// lfi = Line Feed Index
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

func (r *reader) handleTrailingCR(p []byte, n int) (int, error) {
	// n > 0
	r.sp.trailingCR = false
	if p[0] == '\n' {
		r.sp.buf.Write(p[1:n])
		r.markReaderEOF()
		return 0, io.EOF
	}

	r.sp.buf.Write(p[:n])
	p[0] = '\r'
	return 1, nil
}

func (r *reader) handleReadError(p []byte, n int, err error) (int, error) {
	switch n {
	case 0:
		r.err = err
		r.sp.srcErr = err
		r.sp.activeRdr = nil

		if r.sp.trailingCR {
			r.sp.trailingCR = false
			p[0] = '\r'
			return 1, err
		}

		return 0, err
	case 1:
		if p[0] == '\n' {
			r.sp.trailingCR = false
			r.err = err
			r.sp.srcErr = err
			return 0, err
		}
	default:
	}

	// Rather than repeat logic found in reader.Read, we write the unread data
	// to the buffer, set the source reader to an errReader (that always returns
	// err), and then we call r.Read again. This will result in the buffer being
	// read first, followed by the source reader returning an error.
	r.sp.buf.Write(p[:n])
	r.sp.src = errReader{err: err}
	return r.Read(p)
}

func (r *reader) handleBuf(p []byte) (n int, err error) {
	if r.sp.buf.Len() == 0 {
		panic("buf is empty") // FIXME: delete when done with dev
	}

	data := r.sp.buf.Bytes()
	if len(data) > len(p) {
		data = data[:len(p)]
	}

	if r.sp.trailingCR {
		r.sp.trailingCR = false
		if data[0] == '\n' {
			_, _ = r.sp.buf.ReadByte()
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
			n, err = r.sp.buf.Read(p)
			r.sp.trailingCR = true
			_, _ = r.sp.buf.ReadByte() // discard the CR
			return n, err
		}

		n, err = r.sp.buf.Read(p)
		return n, err

	case lfi == 0:
		_, _ = r.sp.buf.ReadByte()
		r.markReaderEOF()
		return 0, io.EOF

	case lfi == 1:
		if data[0] == '\r' {
			_, _ = r.sp.buf.ReadByte() // discard the CR
			_, _ = r.sp.buf.ReadByte() // discard the LF
			r.markReaderEOF()
			return 0, io.EOF
		}

		p[0], _ = r.sp.buf.ReadByte()
		_, _ = r.sp.buf.ReadByte() // discard the LF
		r.markReaderEOF()
		return 1, io.EOF

	default:
		// lfi > 1, continues below
		if data[lfi-1] == '\r' {
			p = p[:lfi-1]
			n, err = r.sp.buf.Read(p)
			_, _ = r.sp.buf.ReadByte() // discard the CR
			_, _ = r.sp.buf.ReadByte() // discard the LF
			r.markReaderEOF()
			return n, err
		}

		p = p[:lfi]
		n, err = r.sp.buf.Read(p)
		_, _ = r.sp.buf.ReadByte() // discard the LF
		r.markReaderEOF()
		return n, err
	}
}

// markReaderEOF marks the reader as EOF and sets the active reader to nil.
func (r *reader) markReaderEOF() {
	r.sp.activeRdr = nil
	r.err = io.EOF
}

func (r *reader) handleLeadingLF(p []byte, srcN int) (n int, err error) {
	if r.sp.trailingCR {
		r.sp.trailingCR = false
	}

	r.markReaderEOF()
	r.sp.buf.Write(p[1:srcN])
	return 0, io.EOF
}

func (r *reader) handleNoLF(p []byte, n int) (int, error) {
	if p[n-1] == '\r' {
		n--
		r.sp.trailingCR = true
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
	_, _ = r.sp.buf.Write(p[lfi+1 : n])
	r.markReaderEOF()

	if n > 1 && p[lfi-1] == '\r' {
		return lfi - 1, io.EOF
	}

	return lfi, io.EOF
}

// ReadAll reads all lines from the source io.Reader and returns them as a slice
// of string. On success, err is nil. That is to say, like io.ReadAll, this
// function does not return io.EOF as an error.
func ReadAll(src io.Reader) (lines []string, err error) {
	sc := New(src)
	var b []byte
	for sc.HasMore() {
		r := sc.Reader()
		if r == nil {
			break
		}

		b, err = io.ReadAll(r)
		if b != nil {
			lines = append(lines, string(b))
		}
		if err != nil {
			break
		}
	}

	return lines, err
}

var _ io.Reader = (*errReader)(nil)

// errReader is an io.Reader that always returns an error.
type errReader struct {
	err error
}

// Read implements [io.Reader]: it always returns errReader.err.
func (e errReader) Read([]byte) (n int, err error) {
	return 0, e.err
}
