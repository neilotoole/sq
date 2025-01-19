// Package linesplitreaders provides a Splitter that returns an io.Reader for
// each line in the source io.Reader. The end goal is similar to bufio.Scanner,
// but Splitter can process arbitrarily long lines, while bufio.Scanner is
// limited to the maximum size of the buffer (defaulting to bufio.MaxScanTokenSize).
package linesplitreaders

import (
	"bytes"
	"io"
)

// Splitter provides a mechanism to return an io.Reader for each line in the
// source io.Reader provided to New. Use Splitter.Next and Splitter.Reader to
// iterate. Lines are split on '\n' or '\r\n' boundaries. A standalone '\r' is
// treated as regular input. At least one reader is returned, even for an empty
// source, with an additional reader returned for newline. Thus, a source
// containing just one newline will result in two readers.
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

// Next returns false if there are no more readers available via Splitter.Reader.
// Note that Next returns true at least once, even if the source io.Reader
// returns zero bytes.
func (s *Splitter) Next() bool {
	return s.srcErr == nil
}

// Reader returns the reader for the next line, or nil if there are no more
// readers available. Reader will panic if the previous reader returned by this
// method has not been fully consumed.
func (s *Splitter) Reader() io.Reader {
	if !s.Next() {
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

// handleReadError is invoked if the source reader returns an error.
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

// handleBuf is invoked from Read when there's data in the buffer.
func (r *reader) handleBuf(p []byte) (n int, err error) {
	data := r.sp.buf.Bytes()
	if len(data) > len(p) {
		data = data[:len(p)]
	}

	if r.sp.trailingCR {
		r.sp.trailingCR = false
		if data[0] == '\n' {
			_, _ = r.sp.buf.ReadByte() // discard the LF
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

func (r *reader) handleLeadingLF(p []byte, n int) (int, error) {
	if r.sp.trailingCR {
		r.sp.trailingCR = false
	}

	r.markReaderEOF()
	_, _ = r.sp.buf.Write(p[1:n])
	return 0, io.EOF
}

func (r *reader) handleNoLF(p []byte, n int) (int, error) {
	if p[n-1] == '\r' {
		n--
		r.sp.trailingCR = true
	}

	return n, nil
}

func (r *reader) handleTrailingLF(p []byte, n int) (int, error) {
	if n > 1 && p[n-2] == '\r' {
		n--
	}

	r.markReaderEOF()
	return n - 1, io.EOF
}

func (r *reader) handleMiddleLF(p []byte, n, lfi int) (int, error) {
	_, _ = r.sp.buf.Write(p[lfi+1 : n])
	r.markReaderEOF()

	if n > 1 && p[lfi-1] == '\r' {
		return lfi - 1, io.EOF
	}

	return lfi, io.EOF
}

// markReaderEOF sets reader.err to io.EOF, and sets Splitter.activeRdr to nil.
func (r *reader) markReaderEOF() {
	r.sp.activeRdr = nil
	r.err = io.EOF
}

// ReadAllStrings is a utility function that reads all lines from the source
// io.Reader and returns them as a slice of string. On success, err is nil.
// Like io.ReadAll, this function does not return io.EOF as an error.
func ReadAllStrings(src io.Reader) (lines []string, err error) {
	sc := New(src)
	var b []byte
	for sc.Next() {
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

// ReadAllBytes is a utility function that reads all lines from the source
// io.Reader and returns them as a slice of []byte. On success, err is nil.
// Like io.ReadAll, this function does not return io.EOF as an error.
func ReadAllBytes(src io.Reader) (lines [][]byte, err error) {
	sc := New(src)
	var b []byte
	for sc.Next() {
		r := sc.Reader()
		if r == nil {
			break
		}

		b, err = io.ReadAll(r)
		if b != nil {
			lines = append(lines, b)
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

// Read implements io.Reader: it always returns errReader.err.
func (e errReader) Read([]byte) (n int, err error) {
	return 0, e.err
}
