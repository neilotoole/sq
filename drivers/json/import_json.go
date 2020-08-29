package json

import (
	"bytes"
	"context"
	stdj "encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func importJSON(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	log.Warn("not implemented")
	return nil
}

// scanObjectsInArray is a convenience function
// for objectsInArrayScanner.
func scanObjectsInArray(r io.Reader) (objs []map[string]interface{}, chunks [][]byte, err error) {
	sc := newObjectInArrayScanner(r)

	for {
		var obj map[string]interface{}
		var chunk []byte

		obj, chunk, err = sc.next()
		if err != nil {
			return nil, nil, err
		}

		if obj == nil {
			// No more objects to be scanned
			break
		}

		objs = append(objs, obj)
		chunks = append(chunks, chunk)
	}

	return objs, chunks, nil
}

// objectsInArrayScanner scans JSON text that consists of an array of
// JSON objects, returning the decoded object and the chunk of JSON
// that it was scanned from. Example input: [{a:1},{a:2},{a:3}]
type objectsInArrayScanner struct {
	// buf will get all the data that the JSON decoder reads.
	// buf's role is to keep track of JSON text that has already been
	// consumed by dec, so that we can return the raw JSON chunk
	// when returning its object. Note that on each invocation
	// of next, the buffer is trimmed, otherwise it would grow
	// unbounded.
	buf *buffer

	// bufOffset is the offset of buf's byte slice relative to the
	// entire input stream. This is necessary given that we trim
	// buf to prevent unbounded growth.
	bufOffset int

	// dec is the stdlib json decoder.
	dec *stdj.Decoder

	// curDecPos is the current position of the decoder in the
	// input stream.
	curDecPos int

	// prevDecPos holds the previous position of the decoder
	// in the input stream.
	prevDecPos int

	// decBuf holds the value of dec.Buffered, which allows us
	// to look ahead at the upcoming decoder values.
	decBuf []byte
}

// newObjectInArrayScanner returns a new instance that
// reads from r.
func newObjectInArrayScanner(r io.Reader) *objectsInArrayScanner {
	buf := &buffer{b: []byte{}}
	// Everything that dec reads from r is written
	// to buf via the TeeReader.
	dec := stdj.NewDecoder(io.TeeReader(r, buf))

	return &objectsInArrayScanner{buf: buf, dec: dec}
}

// next scans the next object from the reader. The returned chunk holds
// the raw JSON that the obj was decoded from. When no more objects,
// obj and chunk are nil.
func (s *objectsInArrayScanner) next() (obj map[string]interface{}, chunk []byte, err error) {
	var tok stdj.Token

	if s.bufOffset == 0 {
		// This is only invoked on the first call to next().

		// The first token must be left-bracket'['
		tok, err = requireDelimToken(s.dec, '[')
		if err != nil {
			return nil, nil, err
		}

		s.prevDecPos = int(s.dec.InputOffset())

		// Sync s.buf with the position in the stream
		s.buf.b = s.buf.b[s.prevDecPos:]
		s.bufOffset = s.prevDecPos
	}

	more := s.dec.More()
	if !more {
		// We've reached the end of the stream.

		// Make sure there's no trailing invalid stuff
		s.decBuf, err = ioutil.ReadAll(s.dec.Buffered())
		if err != nil {
			return nil, nil, errz.Err(err)
		}

		trimmed := bytes.TrimSpace(s.decBuf)
		switch {
		case len(trimmed) == 0:
		case len(trimmed) == 1 && trimmed[0] == ']':
		default:
			return nil, nil, errz.Errorf("invalid JSON: non-whitespace trailing input: %s", string(s.decBuf))
		}

		return nil, nil, nil
	}

	// Decode the next json into obj.
	err = s.dec.Decode(&obj)
	if err != nil {
		return nil, nil, errz.Err(err)
	}

	s.curDecPos = int(s.dec.InputOffset())
	s.decBuf, err = ioutil.ReadAll(s.dec.Buffered())
	if err != nil {
		return nil, nil, errz.Err(err)
	}

	more = s.dec.More()

	// Peek ahead in the decoder buffer
	delimIndex, delim := nextDelim(s.decBuf, 0, true)
	if delimIndex == -1 {
		return nil, nil, errz.New("invalid JSON: additional input expected")
	}

	// If end of input, delim should be right-bracket.
	// If there's another object to come, delim should be comma.
	// If delim not found, or some other delim, it's an error.
	switch delim {
	default:
		// bad input
		return nil, nil, errz.Errorf("invalid JSON: expected comma or right-bracket ']' token but got: %s", formatToken(tok))

	case ']':
		// should be end of input
		tok, err = requireDelimToken(s.dec, ']')
		if err != nil {
			return nil, nil, errz.Err(err)
		}

		if more {
			return nil, nil, errz.New("unexpected additional JSON input after closing ']'")
		}

		// Make sure there's no invalid trailing stuff
		s.decBuf, err = ioutil.ReadAll(s.dec.Buffered())
		if err != nil {
			return nil, nil, errz.Err(err)
		}

		if len(bytes.TrimSpace(s.decBuf)) != 0 {
			return nil, nil, errz.Errorf("invalid JSON: non-whitespace trailing input: %s", string(s.decBuf))
		}

	case ',':
		// Expect more objects to come
		if !more {
			return nil, nil, errz.New("invalid JSON: expected additional tokens input after comma")
		}
	}

	// Note that we re-use the vars delimIndex and delim here.
	// Above us, these vars referred to s.decBuf, not s.buf as here.
	delimIndex, delim = nextDelim(s.buf.b, s.prevDecPos-s.bufOffset, false)
	if delimIndex == -1 {
		return nil, nil, errz.Errorf("invalid JSON: expected delimiter token")
	}

	switch delim {
	default:
		return nil, nil,
			errz.Errorf("invalid JSON: expected comma or left-brace '{' but got: %s", string(delim))
	case '{':
	}

	chunk = make([]byte, s.curDecPos-s.prevDecPos-delimIndex)
	copy(chunk, s.buf.b[s.prevDecPos-s.bufOffset+delimIndex:s.curDecPos-s.bufOffset])

	if !stdj.Valid(chunk) {
		// Should never happen; should be able to delete this check
		return nil, nil, errz.Errorf("invalid JSON")
	}

	// If processing a large stream, s.buf.b will grow unbounded.
	// So we snip it down to size and use s.bufOffset to track
	// the nominal size.
	s.buf.b = s.buf.b[s.curDecPos-s.bufOffset:]
	s.bufOffset = s.curDecPos
	s.prevDecPos = s.curDecPos

	return obj, chunk, nil
}

// buffer is a basic implementation of io.Writer.
type buffer struct {
	b []byte
}

// Write implements io.Writer.
func (b *buffer) Write(p []byte) (n int, err error) {
	b.b = append(b.b, p...)
	return len(p), nil
}

// requireDelimToken invokes dec.Token, returning an error if the
// token is not a delimiter with value delim.
func requireDelimToken(dec *stdj.Decoder, delim rune) (stdj.Token, error) {
	tok, err := dec.Token()
	if err != nil {
		return tok, err
	}

	if tok != stdj.Delim(delim) {
		return tok, errz.Errorf("expected next token to be delimiter %q but got: %s", string(delim), formatToken(tok))
	}

	return tok, nil
}

// formatToken returns a string representation of tok.
func formatToken(tok stdj.Token) string {
	switch v := tok.(type) {
	case string:
		return v
	case stdj.Delim:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// nextDelim returns the index in b of the first JSON
// delimiter (left or right bracket, left or right brace)
// occurring from index start onwards. If arg comma is true,
// comma is also considered a delimiter. If no delimiter
// found, (-1, 0) is returned.
func nextDelim(b []byte, start int, comma bool) (i int, delim byte) {
	for i = start; i < len(b); i++ {
		switch b[i] {
		case '{', '}', '[', ']':
			return i, b[i]
		case ',':
			if comma {
				return i, b[i]
			}
		}
	}

	return -1, 0
}
