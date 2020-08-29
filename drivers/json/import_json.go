package json

import (
	"bytes"
	"context"
	stdj "encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
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
// chunks slice holds the chunk of raw JSON for each object.
func ParseObjectsInArray(r io.Reader) (objs []map[string]interface{}, chunks [][]byte, err error) {
	buf := &buffer{b: []byte{}}
	canonBuf := &buffer{b: []byte{}}
	var bufOffset int

	checkBufOffset := func() {
		if len(buf.b)+bufOffset != len(canonBuf.b) {
			panic(fmt.Sprintf("len(buf): %d  |  bufOffset: %d  | len(canonBuf): %d",
				len(buf.b), bufOffset, len(canonBuf.b)))
		}

		for i := 0; i < len(buf.b); i++ {
			if buf.b[i] != canonBuf.b[i+bufOffset] {
				panic("elements don't match")
			}
		}
	}

	checkBufOffset()

	r = io.TeeReader(r, io.MultiWriter(buf, canonBuf))

	dec := stdj.NewDecoder(r)

	var tok stdj.Token

	var prevDecPos int

	// curDecPos is the position of the decoder in the input stream.
	var curDecPos int

	tok, err = dec.Token()
	if err != nil {
		return nil, nil, errz.Err(err)
	}

	if tok != stdj.Delim('[') {
		return nil, nil, errz.Errorf("expected first delimiter to be left-bracket but got: %s", tokstr(tok))
	}

	prevDecPos = int(dec.InputOffset())
	buf.b = buf.b[prevDecPos:]
	bufOffset = prevDecPos
	checkBufOffset()

	var more bool
	var decBuf []byte
	//var delimIndex int
	//var delim byte

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

		// If there's another object, bufDelim should be comma.
		// If end of input, bufDelim should be right-bracket.
		// If no bufDelim, or some other bufDelim, it's an error.

		// Peek ahead in the decoder buffer
		delimIndex, delim := NextDelim(decBuf, 0)
		if delimIndex == -1 {
			return nil, nil, errz.New("invalid JSON: additional input expected")
		}

		more = dec.More()

		switch delim {
		default:
			// bad input
			return nil, nil, errz.Errorf("invalid JSON: expected comma or right-bracket ']' token but got: %s", tokstr(tok))

		case ']':
			// should be end of input
			tok, err = requireDelimToken(dec, ']')
			if err != nil {
				return nil, nil, errz.Err(err)
			}

			if more {
				return nil, nil, errz.New("unexpected additional JSON input after closing ']'")
			}

			// Make sure there's definitely no invalid trailing stuff
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

		// Now we need to get the chunk for the most recently
		// decoded object.

		checkBufOffset()

		bufDelimIndex, bufDelim := NextDelimNoComma(buf.b, prevDecPos-bufOffset)
		if bufDelimIndex == -1 {
			return nil, nil, errz.Errorf("invalid JSON: expected delimiter token")
		}

		switch bufDelim {
		default:
			return nil, nil, errz.Errorf("invalid JSON: expected comma or left-brace '{' but got: %s", string(bufDelim))
		case ',':
			panic("shouldn't happen, no more comma")

		case '{':
		}

		canonChunk := canonBuf.b[prevDecPos+bufDelimIndex : curDecPos]
		println("canCnk>>>" + string(canonChunk) + "<<<")
		if strings.TrimSpace(string(canonChunk)) != string(canonChunk) {
			panic("canonChunk has whitespace")
		}
		if !stdj.Valid(canonChunk) {
			// Should never happen; should be able to delete this check
			return nil, nil, errz.Errorf("invalid JSON")
		}

		//chunk2 := buf.b[prevDecPos-bufOffset+bufDelimIndex : curDecPos-bufOffset]
		chunk2 := buf.b[prevDecPos-bufOffset+bufDelimIndex : curDecPos-bufOffset]

		println("len chunk2: " + strconv.Itoa(len(chunk2)))
		println("chunk2>>>" + string(chunk2) + "<<<")

		//copy(chunk, buf.b[bufDelimIndex:curDecPos-bufOffset])
		//chunk := make([]byte, len(chunk2))
		//chunkSize := curDecPos - prevDecPos - bufDelimIndex
		chunk := make([]byte, curDecPos-prevDecPos-bufDelimIndex)
		copied := copy(chunk, buf.b[prevDecPos-bufOffset+bufDelimIndex:curDecPos-bufOffset])
		if copied != len(chunk2) {
			panic("bad copy length")
		}
		//copy(chunk, buf.b[bufDelimIndex:curDecPos-bufOffset])
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

		canonReslice := canonBuf.b[curDecPos:]
		println("can reslice>>>" + string(canonReslice) + "<<<")

		println("prior to buf reslice: bufDelim is: [" + strconv.Itoa(bufDelimIndex) + "]  -->  " + string(bufDelim))
		println("buf before>>>" + string(buf.b) + "<<<")
		// Trim the front of the buffer, otherwise it will grow unbounded.

		off := curDecPos - bufOffset
		buf.b = buf.b[off:]
		println("buf after >>>" + string(buf.b) + "<<<")
		bufOffset += off
		checkBufOffset()
		prevDecPos = curDecPos

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

func NextDelimNoComma(b []byte, start int) (i int, delim byte) {
	i, delim = NextDelim(b, start)
	if delim == ',' {
		return NextDelim(b, i+1)
	}

	return i, delim
}
