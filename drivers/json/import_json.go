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

func ScanObjectsInArray(r io.Reader) (objs []map[string]interface{}, chunks [][]byte, err error) {
	sc := newObjectInArrayScanner(r)

	for {
		var obj map[string]interface{}
		var chunk []byte

		obj, chunk, err = sc.next()
		if err != nil {
			return nil, nil, err
		}

		if obj == nil {
			break
		}

		objs = append(objs, obj)
		chunks = append(chunks, chunk)
	}

	return objs, chunks, nil
}

type objectInArrayScanner struct {
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
func newObjectInArrayScanner(r io.Reader) *objectInArrayScanner {
	buf := &buffer{b: []byte{}}
	dec := stdj.NewDecoder(io.TeeReader(r, buf))

	return &objectInArrayScanner{buf: buf, dec: dec}
}

// next scans the next object from the reader. The returned chunk holds
// the raw JSON that the obj was decoded from. When no more objects,
// obj and chunk are nil.
func (s *objectInArrayScanner) next() (obj map[string]interface{}, chunk []byte, err error) {
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
	delimIndex, delim := nextDelim(s.decBuf, 0)
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
	delimIndex, delim = nextDelimNoComma(s.buf.b, s.prevDecPos-s.bufOffset)
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

// ParseObjectsInArray parses JSON that consists of an array of
// JSON objects. For example: [{a:1},{a:2},{a:3}]. The returned
// chunks slice holds the chunk of raw JSON for each object.
func ParseObjectsInArray(r io.Reader) (objs []map[string]interface{}, chunks [][]byte, err error) {
	// buf will get all the data that the JSON decoder reads.
	// buf's role is to keep track of JSON text that has already been
	// consumed by dec, so that we can return the raw JSON chunk
	// when returning its object.
	buf := &buffer{b: []byte{}}
	dec := stdj.NewDecoder(io.TeeReader(r, buf))

	var (
		// bufOffset is the offset of buf's byte slice relative to the
		// entire input stream.
		bufOffset int

		// tok is the JSON token returned by the decoder.
		tok stdj.Token

		// curDecPos is the current position of the decoder in the
		// input stream.
		curDecPos int

		// prevDecPos holds the previous position of the decoder
		// in the input stream.
		prevDecPos int

		// more holds the value of dec.More, indicating if there
		// are more tokens remaining to be decoded.
		more bool

		// decBuf holds the value of dec.Buffered, which allows us
		// to look ahead at the upcoming decoder values.
		decBuf []byte

		// delimIndex and delim are the index and value of the next
		// delimiters in either buf or decBuf.
		delimIndex int
		delim      byte
	)

	// The first token must be left-bracket'['
	tok, err = requireDelimToken(dec, '[')
	if err != nil {
		return nil, nil, err
	}

	prevDecPos = int(dec.InputOffset())

	// Sync buf with the position in the stream
	buf.b = buf.b[prevDecPos:]
	bufOffset = prevDecPos

	for {
		more = dec.More()
		if !more {
			// Make sure there's no trailing invalid stuff
			decBuf, err = ioutil.ReadAll(dec.Buffered())
			if err != nil {
				return nil, nil, errz.Err(err)
			}

			trimmed := bytes.TrimSpace(decBuf)
			if len(trimmed) == 0 {
				break
			}

			if len(trimmed) == 1 && trimmed[0] == ']' {
				break
			}

			return nil, nil, errz.Errorf("invalid JSON: non-whitespace trailing input: %s", string(decBuf))
		}

		var m map[string]interface{}
		err = dec.Decode(&m)
		if err != nil {
			return nil, nil, errz.Err(err)
		}
		objs = append(objs, m)

		curDecPos = int(dec.InputOffset())

		decBuf, err = ioutil.ReadAll(dec.Buffered())
		if err != nil {
			return nil, nil, errz.Err(err)
		}

		more = dec.More()

		// Peek ahead in the decoder buffer
		delimIndex, delim = nextDelim(decBuf, 0)
		if delimIndex == -1 {
			return nil, nil, errz.New("invalid JSON: additional input expected")
		}

		// If end of input, delim should be right-bracket.
		// If there's another object, delim should be comma.
		// If delim not found, or some other delim, it's an error.
		switch delim {
		default:
			// bad input
			return nil, nil, errz.Errorf("invalid JSON: expected comma or right-bracket ']' token but got: %s", formatToken(tok))

		case ']':
			// should be end of input
			tok, err = requireDelimToken(dec, ']')
			if err != nil {
				return nil, nil, errz.Err(err)
			}

			if more {
				return nil, nil, errz.New("unexpected additional JSON input after closing ']'")
			}

			// Make sure there's no invalid trailing stuff
			decBuf, err = ioutil.ReadAll(dec.Buffered())
			if err != nil {
				return nil, nil, errz.Err(err)
			}

			if len(bytes.TrimSpace(decBuf)) != 0 {
				return nil, nil, errz.Errorf("invalid JSON: non-whitespace trailing input: %s", string(decBuf))
			}

		case ',':
			// Expect more objects to come
			if !more {
				return nil, nil, errz.New("invalid JSON: expected additional tokens input after comma")
			}
		}

		// Note that we are re-using the vars delimIndex and delim here.
		// Above us, these vars referred to the value in decBuf, not buf as here.
		delimIndex, delim = nextDelimNoComma(buf.b, prevDecPos-bufOffset)
		if delimIndex == -1 {
			return nil, nil, errz.Errorf("invalid JSON: expected delimiter token")
		}

		switch delim {
		default:
			return nil, nil,
				errz.Errorf("invalid JSON: expected comma or left-brace '{' but got: %s", string(delim))
		case '{':
		}

		chunk := make([]byte, curDecPos-prevDecPos-delimIndex)
		copy(chunk, buf.b[prevDecPos-bufOffset+delimIndex:curDecPos-bufOffset])

		if !stdj.Valid(chunk) {
			// Should never happen; should be able to delete this check
			return nil, nil, errz.Errorf("invalid JSON")
		}

		// If processing a large stream, buf.b will grow unbounded.
		// So we snip it down to size and use bufOffset to track
		// the nominal size.
		buf.b = buf.b[curDecPos-bufOffset:]
		bufOffset = curDecPos
		prevDecPos = curDecPos

		chunks = append(chunks, chunk)
	}

	return objs, chunks, nil
}

// buffer is a trivial implementation of io.Writer.
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
// delimiter (left or right bracket, left or right brace, or comma)
// occurring from index start onwards. If no delimiter found,
// (-1, 0) is returned. Note that unlike dec.Token, this function
// will return comma.
func nextDelim(b []byte, start int) (i int, delim byte) {
	for i = start; i < len(b); i++ {
		switch b[i] {
		case ',', '{', '}', '[', ']':
			return i, b[i]
		}
	}

	return -1, 0
}

// nextDelimNoComma works like nextDelim but skips comma (thus
// it works like the stdlib json decoder).
func nextDelimNoComma(b []byte, start int) (i int, delim byte) {
	for i = start; i < len(b); i++ {
		switch b[i] {
		case '{', '}', '[', ']':
			return i, b[i]
		}
	}

	return -1, 0
}
