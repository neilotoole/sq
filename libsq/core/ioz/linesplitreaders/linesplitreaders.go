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

	// advanceN is set to true when the current a new line is available.
	advanceN  bool
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
	//
	//
	//
	//if errors.Is(err, io.EOF) {
	//	if n == 0 {
	//		return 0, io.EOF
	//	}
	//
	//	if p[n-1] == '\r' {
	//		r.sc.trailingR = true
	//	}
	//
	//	return n, nil
	//}
	//
	//return n, err
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
		//r.sc.srcErr = err
		//
		//if errors.Is(err, io.EOF) {
		//
		//}
		//
		//return n, err
	}

	if n == 0 { // FIXME: revisit
		return 0, nil
	}

	// n >= 1
	if r.sc.trailingR {
		r.sc.trailingR = false
		if n == 1 {
			if p[0] == '\r' {
				r.sc.trailingR = true
				return 1, nil
			}
			if p[0] == '\n' {
				r.markReaderEOF()
				return 0, io.EOF
			}
			// Else, it's a regular character
			r.sc.buf.WriteByte(p[0])
			p[0] = '\r'
			return 1, nil
		}

		// Else, n > 1
		if p[0] == '\n' {
			r.sc.buf.Write(p[1:n])
			r.markReaderEOF()
			return 0, io.EOF
		}
		if p[0] == '\r' {
			r.sc.trailingR = true
			r.sc.buf.Write(p[1:n])
			return 1, nil
		}

		r.sc.buf.Write(p[1:n])
		p[0] = '\r'
		return 1, nil
	}

	data := p[:n]
	lfi := bytes.IndexByte(data, '\n')

	// Options
	// 1. Leading newline
	// 2. No newline
	// 3. Trailing newline
	// 4. Middle newline

	switch {
	case lfi == 0: // Leading newline
		return r.handleLeadingLF(p, n)

	case lfi < 0: // Didn't find a newline
		return r.handleNoLF(p, data, n)

	case lfi == n-1: // trailing newline
		return r.handleTrailingLF(p, n)

	default:
		return r.handleMiddleLF(p, n, lfi)
	}
}

func (r *reader) handleBuf(p []byte) (n int, err error) {
	if r.sc.buf.Len() == 0 {
		panic("buf is empty")
	}

	if r.sc.advanceN {
		r.markReaderEOF()
		r.sc.advanceN = false
		return 0, io.EOF
	}

	lenBufBytes := r.sc.buf.Len()
	_ = lenBufBytes
	data := r.sc.buf.Bytes()
	if len(data) > len(p) {
		data = data[:len(p)]
	}

	lfi := bytes.IndexByte(data, '\n')
	switch {
	case lfi < 0:
		if r.sc.buf.Len() == 1 {
			b, _ := r.sc.buf.ReadByte()
			if b == '\r' {
				return 0, nil
			}

			_ = r.sc.buf.UnreadByte()
		}

		n, err = r.sc.buf.Read(p)
		if n > 0 && p[n-1] == '\r' {
			n--
		}
		return n, err
	case lfi == 0:
		r.sc.advanceN = true
		_, _ = r.sc.buf.ReadByte() // Discard the LF
		return 0, nil
	// case lfi == len(data)-1:

	default:
		if lfi > 0 && data[lfi-1] == '\r' {
			p = p[:lfi-1]
			n, err = r.sc.buf.Read(p)
			_, _ = r.sc.buf.ReadByte() // discard the CR
		} else {
			p = p[:lfi]
			n, err = r.sc.buf.Read(p)
		}

		return n, err
	}
}

func (r *reader) markReaderEOF() {
	r.sc.activeRdr = nil
	r.err = io.EOF
}

func (r *reader) handleLeadingLF(p []byte, srcN int) (n int, err error) {
	if r.sc.advanceN {
		r.markReaderEOF()
		_, _ = r.sc.buf.Write(p[1:srcN])
		r.sc.advanceN = true
		return 0, io.EOF
	}

	if srcN == 1 {
		r.sc.advanceN = true
		return 0, nil
	}

	r.sc.advanceN = true
	_, _ = r.sc.buf.Write(p[1:srcN])

	return 0, nil
}

func (r *reader) handleNoLF(p, data []byte, srcN int) (n int, err error) {
	if srcN == 1 && data[0] == '\r' {
		// Special case, discard single CR
		r.sc.trailingR = true
		return 0, nil
	}

	if srcN > 0 && data[srcN-1] == '\r' {
		srcN--
	}

	if r.sc.advanceN {
		r.sc.advanceN = false
		r.markReaderEOF()
		_, _ = r.sc.buf.Write(p[:srcN])
		return 0, io.EOF
	}

	return srcN, nil
}

func (r *reader) handleTrailingLF(p []byte, srcN int) (n int, err error) {
	if r.sc.advanceN {
		r.markReaderEOF()
		r.sc.advanceN = false
		_, _ = r.sc.buf.Write(p[:srcN])
		return 0, io.EOF
	}

	r.sc.advanceN = true
	if srcN > 1 && p[srcN-2] == '\r' {
		srcN--
	}

	return srcN - 1, nil
}

func (r *reader) handleMiddleLF(p []byte, srcN, lfi int) (n int, err error) {
	if r.sc.advanceN {
		r.sc.advanceN = false
		r.markReaderEOF()
		_, _ = r.sc.buf.Write(p[:srcN])
		return 0, io.EOF
	}

	_, _ = r.sc.buf.Write(p[lfi+1 : srcN])

	r.markReaderEOF()

	if srcN > 1 && p[lfi-1] == '\r' {
		return lfi - 1, io.EOF
	}

	return lfi, io.EOF
}

//func Items() iter.Seq[Item] {
//	return func(yield func(Item) bool) {
//		items := []Item{1, 2, 3}
//		for _, v := range items {
//			if !yield(v) {
//				return
//			}
//		}
//	}
//}

//func Lines(src io.Reader) iter.Seq[io.Reader] {
//	splitter := New(src)
//
//	return func(yield func(src io.Reader) bool) {
//		items := []Item{1, 2, 3}
//		for _, v := range items {
//			if !yield(v) {
//				return
//			}
//		}
//	}
//}
