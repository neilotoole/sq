package json

import (
	"bytes"
	"context"
	stdj "encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
)

// DetectJSON returns a source.DriverDetectFunc that can detect JSON.
func DetectJSON(sampleSize int) source.DriverDetectFunc { // FIXME: is DetectJSON actually working?
	return func(ctx context.Context, openFn source.NewReaderFunc) (detected drivertype.Type, score float32,
		err error,
	) {
		log := lg.FromContext(ctx)
		start := time.Now()
		defer func() {
			log.Debug("JSON detection complete", lga.Elapsed, time.Since(start), lga.Score, score)
		}()

		var r1, r2 io.ReadCloser
		r1, err = openFn(ctx)
		if err != nil {
			return drivertype.None, 0, errz.Err(err)
		}
		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r1)

		dec := stdj.NewDecoder(r1)
		var tok stdj.Token
		tok, err = dec.Token()
		if err != nil {
			return drivertype.None, 0, nil
		}

		delim, ok := tok.(stdj.Delim)
		if !ok {
			return drivertype.None, 0, nil
		}

		switch delim {
		default:
			return drivertype.None, 0, nil
		case leftBrace:
			// The input is a single JSON object
			r2, err = openFn(ctx)

			// buf gets a copy of what is read from r2
			buf := &buffer{}

			if err != nil {
				return drivertype.None, 0, errz.Err(err)
			}
			defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r2)

			dec = stdj.NewDecoder(io.TeeReader(r2, buf))
			var m map[string]any
			err = dec.Decode(&m)
			if err != nil {
				return drivertype.None, 0, nil
			}

			if dec.More() {
				// The input is supposed to be just a single object, so
				// it shouldn't have more tokens
				return drivertype.None, 0, nil
			}

			// If the input is all on a single line, then it could be
			// either JSON or JSONL. For single-line input, prefer JSONL.
			lineCount := stringz.LineCount(bytes.NewReader(buf.b), true)
			switch lineCount {
			case -1:
				// should never happen
				return drivertype.None, 0, errz.New("unknown problem reading JSON input")
			case 0:
				// should never happen
				return drivertype.None, 0, errz.New("JSON input is empty")
			case 1:
				// If the input is a JSON object on a single line, it could
				// be TypeJSON or TypeJSONL. In deference to TypeJSONL, we
				// return 0.9 instead of 1.0
				return TypeJSON, 0.9, nil
			default:
				return TypeJSON, 1.0, nil
			}

		case leftBracket:
			// The input is one or more JSON objects inside an array
		}

		r2, err = openFn(ctx)
		if err != nil {
			return drivertype.None, 0, errz.Err(err)
		}
		defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r2)

		sc := newObjectInArrayScanner(r2)
		var validObjCount int
		var obj map[string]any

		for {
			select {
			case <-ctx.Done():
				return drivertype.None, 0, ctx.Err()
			default:
			}

			obj, _, err = sc.next()
			if err != nil {
				return drivertype.None, 0, ctx.Err()
			}

			if obj == nil { // end of input
				break
			}

			validObjCount++
			if validObjCount >= sampleSize {
				break
			}
		}

		if validObjCount > 0 {
			return TypeJSON, 1.0, nil
		}

		return drivertype.None, 0, nil
	}
}

func ingestJSON(ctx context.Context, job ingestJob) error {
	log := lg.FromContext(ctx)

	r, err := job.openFn(ctx)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	drvr := job.destGrip.SQLDriver()

	db, err := job.destGrip.DB(ctx)
	if err != nil {
		return err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDB, conn)

	proc := newProcessor(job.flatten)
	scan := newObjectInArrayScanner(r)

	var (
		obj            map[string]any
		chunk          []byte
		schemaModified bool
		curSchema      *importSchema
		insertions     []*insertion
		hasMore        bool
	)

	for {
		obj, chunk, err = scan.next()
		if err != nil {
			return err
		}

		// obj is returned nil by scan.next when end of input.
		hasMore = obj != nil

		if schemaModified {
			if !hasMore || scan.objCount >= job.sampleSize {
				log.Debug("Time to (re)build the schema", lga.Line, scan.objCount)
				if curSchema == nil {
					log.Debug("First time building the schema")
				}

				var newSchema *importSchema
				newSchema, err = proc.buildSchemaFlat()
				if err != nil {
					return err
				}

				err = execSchemaDelta(ctx, drvr, conn, curSchema, newSchema)
				if err != nil {
					return err
				}

				// The DB has been updated with the current schema,
				// so we mark it as clean.
				proc.markSchemaClean()

				curSchema = newSchema
				newSchema = nil

				insertions, err = proc.buildInsertionsFlat(curSchema)
				if err != nil {
					return err
				}

				err = execInsertions(ctx, drvr, conn, insertions)
				if err != nil {
					return err
				}
			}

			if !hasMore {
				// We're done
				break
			}
		}

		schemaModified, err = proc.processObject(obj, chunk)
		if err != nil {
			return err
		}

		// Initial schema has not been created: we're still in
		// the sampling phase. So we loop.
		if curSchema == nil {
			continue
		}

		// If we got this far, the initial schema has already been created.
		if schemaModified {
			// But... the schema has been modified. We could still be in
			// the sampling phase, so we loop.
			continue
		}

		// The schema exists in the DB, and the current JSON chunk hasn't
		// dirtied the schema, so it's safe to insert the recent rows.
		insertions, err = proc.buildInsertionsFlat(curSchema)
		if err != nil {
			return err
		}

		err = execInsertions(ctx, drvr, conn, insertions)
		if err != nil {
			return err
		}
	}

	if scan.objCount == 0 {
		return errz.New("empty JSON input")
	}

	return nil
}

// objectsInArrayScanner scans JSON text that consists of an array of
// JSON objects, returning the decoded object and the chunk of JSON
// that it was scanned from. Example input: [{a:1},{a:2},{a:3}].
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

	// objCount is the count of objects processed by method next.
	objCount int
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
func (s *objectsInArrayScanner) next() (obj map[string]any, chunk []byte, err error) {
	var tok stdj.Token

	if s.bufOffset == 0 {
		// This is only invoked on the first call to next().

		// The first token must be left-bracket '['
		tok, err = requireDelimToken(s.dec, leftBracket)
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
		s.decBuf, err = io.ReadAll(s.dec.Buffered())
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
	s.decBuf, err = io.ReadAll(s.dec.Buffered())
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
		return nil, nil, errz.Errorf("invalid JSON: expected comma or right-bracket ']' token but got: %s",
			formatToken(tok))

	case ']':
		// should be end of input
		_, err = requireDelimToken(s.dec, rightBracket)
		if err != nil {
			return nil, nil, errz.Err(err)
		}

		if more {
			return nil, nil, errz.New("unexpected additional JSON input after closing ']'")
		}

		// Make sure there's no invalid trailing stuff
		s.decBuf, err = io.ReadAll(s.dec.Buffered())
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

	s.objCount++
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
// token is not delim.
func requireDelimToken(dec *stdj.Decoder, delim stdj.Delim) (stdj.Token, error) {
	tok, err := dec.Token()
	if err != nil {
		return tok, err
	}

	if tok != delim {
		return tok, errz.Errorf("expected next token to be delimiter {%s} but got: %s", string(delim), formatToken(tok))
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
