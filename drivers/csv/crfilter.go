package csv

import "io"

// crFilterReader is a reader whose Read method converts
// standalone carriage return '\r' bytes to newline '\n'.
// CRLF "\r\n" sequences are untouched.
// This is useful for reading from DOS format, e.g. a TSV
// file exported by Microsoft Excel.
type crFilterReader struct {
	r io.Reader
}

func (r *crFilterReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)

	for i := 0; i < n; i++ {
		if p[i] == 13 {
			if i+1 < n && p[i+1] == 10 {
				continue // it's \r\n
			}
			// it's just \r by itself, replace
			p[i] = 10
		}
	}

	return n, err
}
