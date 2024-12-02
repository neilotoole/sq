// Package linesplitreaders provides a type that returns an io.Reader for
// each line in the source io.Reader.
package linesplitreaders

import (
	"bytes"
	"errors"
	"fmt"
	"io"
)

/*

https://stackoverflow.com/questions/37530451/golang-bufio-read-multiline-until-crlf-r-n-delimiter/37531472
*/

type Splitter struct {
	src       io.Reader
	buf       *bytes.Buffer
	srcEOF    bool
	srcErr    error
	activeRdr *reader
	advance   bool
}

func New(src io.Reader) *Splitter {
	return &Splitter{src: src, buf: &bytes.Buffer{}}
}

// Next returns true if there is another reader available Splitter.Reader.
func (s *Splitter) Next() bool {
	return s.srcErr == nil && !s.srcEOF
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

func (r *reader) handleBuf(p []byte) (n int, err error) {
	if r.sc.buf.Len() == 0 {
		panic("buf is empty")
	}

	if r.sc.advance {
		r.markReaderEOF()
		r.sc.advance = false
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
		r.sc.advance = true
		_, _ = r.sc.buf.ReadByte() // Discard the LF
		return 0, nil
	//case lfi == len(data)-1:

	default:
		if lfi > 0 && data[lfi-1] == '\r' {
			p = p[:lfi-1]
			n, err = r.sc.buf.Read(p)
			_, _ = r.sc.buf.ReadByte() // discard the CR
		} else {
			p = p[:lfi]
			n, err = r.sc.buf.Read(p)
		}
		// Somewhere in the middle
		//p = p[:lfi]
		////lr := io.LimitReader(r.sc.buf, int64(lfi))
		////// FIXME: set rc.trailing = true, and advance by one?
		////n, err = lr.Read(p)
		//n, err = r.sc.buf.Read(p)
		return n, err
	}

	//n, err = r.sc.buf.Read(p) // FIXME: do this part
	//if err != nil {
	//	panic(err)
	//}
	//
	//return n, err
}

func (r *reader) markReaderEOF() {
	r.sc.activeRdr = nil
	r.err = io.EOF
}

func (r *reader) handleLeadingLF(p, data []byte, srcN int) (n int, err error) {
	if r.sc.advance {
		r.markReaderEOF()
		_, _ = r.sc.buf.Write(p[1:srcN])
		r.sc.advance = true
		return 0, io.EOF
	}

	if srcN == 1 {
		r.sc.advance = true
		return 0, nil
	}

	r.sc.advance = true
	_, _ = r.sc.buf.Write(p[1:srcN])

	return 0, nil
}

func (r *reader) handleNoLF(p, data []byte, srcN int) (n int, err error) {
	if srcN == 1 && data[0] == '\r' {
		// Special case, discard single CR

		return 0, nil
	}

	if srcN > 0 && data[srcN-1] == '\r' {
		srcN--
	}

	if r.sc.advance {
		r.sc.advance = false
		r.markReaderEOF()
		_, _ = r.sc.buf.Write(p[:srcN])
		return 0, io.EOF
	}

	return srcN, nil
}

func (r *reader) handleTrailingLF(p, data []byte, srcN, lfi int) (n int, err error) {
	if r.sc.advance {
		r.markReaderEOF()
		r.sc.advance = false
		_, _ = r.sc.buf.Write(p[:srcN])
		return 0, io.EOF
	}

	r.sc.advance = true
	if srcN > 1 && p[srcN-2] == '\r' {
		srcN--
	}

	return srcN - 1, nil
}

func (r *reader) handleMiddleLF(p, data []byte, srcN, lfi int) (n int, err error) {
	if r.sc.advance {
		r.sc.advance = false
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

func (r *reader) Read(p []byte) (n int, err error) {
	if r.sc.srcErr != nil {
		return 0, r.sc.srcErr
	}
	if r.err != nil {
		return 0, err
	}

	bufLen := r.sc.buf.Len()
	_ = bufLen

	fmt.Println("bufLen:", bufLen)
	fmt.Printf("buf: >%s<\n", r.sc.buf.String())

	if r.sc.buf.Len() > 0 {
		return r.handleBuf(p)
		//n, err = r.sc.buf.Read(p)
	}

	n, err = r.sc.src.Read(p)

	if err != nil {
		r.sc.srcErr = err
		if errors.Is(err, io.EOF) {
			r.sc.srcEOF = true
		}

		return n, err
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
		return r.handleLeadingLF(p, data, n)
		//if n == 1 {
		//	if r.sc.trailing {
		//		r.sc.activeRdr = nil
		//		return 0, io.EOF
		//	}
		//
		//	r.sc.trailing = true
		//	//r.sc.activeRdr = nil
		//	return 0, nil
		//}
		//
		//_, err = r.sc.buf.Write(p[i+1 : n])
		//if err != nil {
		//	panic(err)
		//	//return i, err
		//}
		//
		//r.sc.activeRdr = nil
		//return 0, io.EOF
	case lfi < 0: // Didn't find a newline
		return r.handleNoLF(p, data, n)
		//i = bytes.IndexByte(data, '\r')
		//if i < 0 {
		//	if r.sc.trailing {
		//		r.sc.trailing = false
		//		r.sc.activeRdr = nil
		//		return n, io.EOF
		//	}
		//
		//	return n, nil
		//}

		//if i == 0 && len(p) == 1 {
		//if lfi == 0 {
		//	r.sc.trailing = true
		//	if n > 1 {
		//		_, err = r.sc.buf.Write(p[1:])
		//		if err != nil {
		//			panic(err)
		//			//return i, err
		//		}
		//	}
		//	return 0, nil
		//}
		//
		//if lfi == n-1 {
		//	//r.done = true
		//	r.sc.trailing = true
		//	return lfi, nil
		//}

	case lfi == n-1: // trailing newline
		return r.handleTrailingLF(p, data, n, lfi)
		// Found a trailing newline
		//if p[n-2] == '\r' {
		//	n--
		//}
		//return n - 1, nil

	default:
		return r.handleMiddleLF(p, data, n, lfi)
	}

	//if p[i-1] == '\r' {
	//	i--
	//}

	//_, err = r.sc.buf.Write(p[lfi+1 : n])
	//if p[lfi-1] == '\r' {
	//	lfi--
	//}
	//if err != nil {
	//	panic(err)
	//	//return i, err
	//}
	//
	//r.sc.activeRdr = nil
	//return lfi, io.EOF
}

//
//func (r *reader) ReadOld(p []byte) (n int, err error) {
//	if r.done {
//		return 0, io.EOF
//	}
//
//	if r.sc.buf.Len() > 0 {
//		n, err = r.sc.buf.Read(p)
//	} else {
//		n, err = r.sc.src.Read(p)
//	}
//
//	if err != nil {
//		r.sc.done = true
//
//		if n == 0 && errors.Is(err, io.EOF) {
//			return 0, io.EOF
//		}
//
//		return n, err
//	}
//
//	data := p[:n]
//	i := bytes.IndexByte(data, '\n')
//
//	switch {
//	case i < 0:
//		// Didn't find a newline
//		i = bytes.IndexByte(data, '\r')
//		if i < 0 {
//			return n, nil
//		}
//
//		//if i == 0 && len(p) == 1 {
//		if i == 0 {
//			r.sc.trailingCR = true
//			if n > 1 {
//				_, err = r.sc.buf.Write(p[1:])
//				if err != nil {
//					return i, err
//				}
//			}
//			return 0, nil
//		}
//
//		if i == n-1 {
//			r.done = true
//			r.sc.trailingCR = true
//			return i, nil
//		}
//	case i == 0:
//		if n == 1 {
//			return 0, io.EOF
//		}
//
//		_, err = r.sc.buf.Write(p[i+1 : n])
//		if err != nil {
//			return i, err
//		}
//
//		return 0, io.EOF
//	default:
//	}
//
//	//if p[i-1] == '\r' {
//	//	i--
//	//}
//
//	_, err = r.sc.buf.Write(p[i+1 : n])
//	if p[i-1] == '\r' {
//		i--
//	}
//	if err != nil {
//		return i, err
//	}
//
//	return i, io.EOF
//}
