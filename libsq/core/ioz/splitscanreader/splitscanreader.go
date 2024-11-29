package splitscanreader

import (
	"bytes"
	"errors"
	"io"
)

/*

https://stackoverflow.com/questions/37530451/golang-bufio-read-multiline-until-crlf-r-n-delimiter/37531472
*/

type Scanner struct {
	src        io.Reader
	buf        *bytes.Buffer
	done       bool
	trailingCR bool
}

func NewScanner(src io.Reader) *Scanner {
	return &Scanner{src: src, buf: &bytes.Buffer{}}
}

func (s *Scanner) Scan() bool {
	return !s.done
}

// Reader returns the next reader, or nil.
func (s *Scanner) Reader() io.Reader {
	if s.done {
		return nil
	}
	return &reader{sc: s}
}

var _ io.Reader = &reader{}

type reader struct {
	sc   *Scanner
	done bool
}

func (r *reader) Read(p []byte) (n int, err error) {
	if r.done {
		return 0, io.EOF
	}

	if r.sc.buf.Len() > 0 {
		n, err = r.sc.buf.Read(p)
	} else {
		n, err = r.sc.src.Read(p)
	}

	if err != nil {
		r.sc.done = true

		if n == 0 && errors.Is(err, io.EOF) {
			return 0, io.EOF
		}

		return n, err
	}

	data := p[:n]
	i := bytes.IndexByte(data, '\n')

	if i < 0 {
		i = bytes.IndexByte(data, '\r')
		if i < 0 {
			return n, nil
		}
		if i == n-1 {
			r.done = true
			r.sc.trailingCR = true
			return i, nil
		}
	}

	if i == 0 {
		if n == 1 {
			return 0, io.EOF
		}

		_, err = r.sc.buf.Write(p[i+1 : n])
		if err != nil {
			return i, err
		}

		return 0, io.EOF
	}

	_, err = r.sc.buf.Write(p[i+1 : n])
	if p[i-1] == '\r' {
		i--
	}
	if err != nil {
		return i, err
	}

	return i, io.EOF
}
