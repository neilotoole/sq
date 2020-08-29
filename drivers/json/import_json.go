package json

import (
	"context"
	stdj "encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

func importJSON(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	log.Warn("not implemented")
	return nil
}

type buffer struct {
	b []byte
}

func (b *buffer) Write(p []byte) (n int, err error) {
	b.b = append(b.b, p...)
	return len(p), nil
}

// ParseObjectsInArray parses JSON that consists of an array of
// JSON objects. For example: [{a:1},{a:2},{a:3}]. The returned
// chunks holds the chunk of raw JSON for each object.
func ParseObjectsInArray(r io.Reader) (objs []map[string]interface{}, chunks [][]byte, err error) {
	buf := &buffer{b: []byte{}}
	r = io.TeeReader(r, buf)
	dec := stdj.NewDecoder(r)

	var tok stdj.Token
	var startOffset, endOffset int

	tok, err = dec.Token()
	if err != nil {
		return nil, nil, errz.Err(err)
	}

	if tok != stdj.Delim('[') {
		return nil, nil, errz.Errorf("expected first delimiter to be left-bracket but got: %s", tokstr(tok))
	}

	startOffset = int(dec.InputOffset())

	var more bool
	var decBuf []byte
	var delimIndex int
	var delim byte
	var bufOffset int

	for {
		more = dec.More()
		if !more {
			break
		}

		var m map[string]interface{}
		err = dec.Decode(&m)
		if err != nil {
			return nil, nil, errz.Err(err)
		}
		objs = append(objs, m)

		endOffset = int(dec.InputOffset())

		decBuf, err = ioutil.ReadAll(dec.Buffered())

		// If there's another object, delim should be comma.
		// If end of input, delim should be right-bracket.
		// If no delim, or some other delim, it's an error.

		// peek ahead in the decoder buffer
		delimIndex, delim = NextDelim(decBuf, 0)
		//if delimIndex == -1 {
		//	return
		//}

		more = dec.More()

		switch delim {
		default:
			// bad input
			return nil, nil, errz.Errorf("invalid JSON: expected comma or right-bracket token but got: %s", tokstr(tok))

		case ']':
			// should be end of input

			tok, err = requireDelimToken(dec, ']')
			if err != nil {
				return nil, nil, errz.Err(err)
			}

			if more {
				return nil, nil, errz.New("unexpected additional JSON input after closing ']'")
			}

			//endChunk := buf.b[startOffset-bufOffset : endOffset-bufOffset]
			//println("endchunk:>>>" + string(endChunk) + "<<<")
			//if !stdj.Valid(endChunk) {
			//	// Should never happen; should be able to delete this check
			//	return nil, nil, errz.Errorf("invalid JSON")
			//}
			//
			//chunks = append(chunks, endChunk)
			//return objs, chunks, nil

		case ',':
			// Expect more objects to come
			if !more {
				return nil, nil, errz.New("invalid JSON: expected additional tokens input after comma")
			}

		}

		// Now we need to get the chunk for the most recently
		// decoded object.

		// We need to advance buf

		// delim could be comma or left-brace
		delimIndex, delim = NextDelim(buf.b, startOffset-bufOffset)
		if delimIndex == -1 {
			return nil, nil, errz.Errorf("invalid JSON: expected delimiter token")
		}

		switch delim {
		default:
			return nil, nil, errz.Errorf("invalid JSON: expected comma or left-brace '{' but got: %s", string(delim))
		case ',':
			println("got the comma")
			// If it's a comma, we need to advance startOffset until we
			// reach left-bracket (which MUST be the next delim).
			//startOffset++
			delimIndex, delim = NextDelim(buf.b, startOffset+1-bufOffset)
			if delimIndex == -1 {
				return nil, nil, errz.Errorf("invalid JSON: expected delimiter token")
			}

			if delim != '{' {
				return nil, nil, errz.Errorf("invalid JSON: expected left-brace '{' token")
			}
			//startOffset = startOffset + delimIndex

		case '{':
		}

		//if delim != '{' {
		//	return nil, nil, errz.Errorf("invalid JSON: expected left-brace delimiter token but got: %s", string(delim))
		//}

		chunkSize := endOffset - delimIndex - bufOffset
		chunk := make([]byte, chunkSize)
		chunk2 := buf.b[delimIndex : endOffset-bufOffset]
		//println("chunk2>>>" + string(chunk2) + "<<<")
		copy(chunk, buf.b[delimIndex:endOffset-bufOffset])
		println("chunk >>>" + string(chunk2) + "<<<")

		if strings.TrimSpace(string(chunk)) != string(chunk) {
			panic("chunk has whitespace")
		}

		if string(chunk) != string(chunk2) {
			panic("chunk != chunk2")
		}

		if !stdj.Valid(chunk) {
			// Should never happen; should be able to delete this check
			return nil, nil, errz.Errorf("invalid JSON")
		}

		// Trim the front of the buffer, otherwise it will grow unbounded.
		buf.b = buf.b[endOffset-bufOffset+delimIndex:]
		bufOffset = endOffset + delimIndex
		startOffset = endOffset + delimIndex

		println("buf after>>>" + string(buf.b) + "<<<")

		//fmt.Fprintf(os.Stdout, "[%d] buf size: len(%d) cap(%d)\n", len(chunks), len(buf.b), cap(buf.b))
		//err = os.Stdout.Sync()
		//if err != nil {
		//	return nil, nil, err
		//}

		chunks = append(chunks, chunk)
	}

	return objs, chunks, nil

}

// requireDelimToken invokes dec.Token, returning an error if the
// token is not a delimiter with value delim.
func requireDelimToken(dec *stdj.Decoder, delim rune) (stdj.Token, error) {
	tok, err := dec.Token()
	if err != nil {
		return tok, err
	}

	if tok != stdj.Delim(delim) {
		return tok, errz.Errorf("expected next token to be delimiter %q but got: %s", string(delim), tokstr(tok))
	}

	return tok, nil
}

// tokstr returns a string representation of tok.
func tokstr(tok stdj.Token) string {
	switch v := tok.(type) {
	case string:
		return v
	case stdj.Delim:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// NextDelim returns the index in b of the first JSON
// delimiter (left or right bracket, left or right brace, or comma)
// occurring from index start onwards. If no delimiter found,
// (-1, 0) is returned.
func NextDelim(b []byte, start int) (i int, delim byte) {
	if start < 0 {
		panic("found it")
	}

	for i = start; i < len(b); i++ {
		switch b[i] {
		case ',', '{', '}', '[', ']':
			return i, b[i]
		}
	}

	return -1, 0
}
