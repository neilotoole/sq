package json

import (
	"bufio"
	"bytes"
	"context"
	stdj "encoding/json"
	"io"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// DetectJSONL implements source.TypeDetectFunc.
func DetectJSONL(ctx context.Context, log lg.Log, openFn source.FileOpenFunc) (detected source.Type, score float32, err error) {
	var r io.ReadCloser
	r, err = openFn()
	if err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}
	defer log.WarnIfCloseError(r)

	sc := bufio.NewScanner(r)
	var validLines int
	var line []byte

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return source.TypeNone, 0, ctx.Err()
		default:
		}

		if err = sc.Err(); err != nil {
			return source.TypeNone, 0, errz.Err(err)
		}

		line = sc.Bytes()
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			// Probably want to skip blank lines? Maybe
			continue
		}

		// Each line of JSONL must be braced
		if line[0] != '{' || line[len(line)-1] != '}' {
			return source.TypeNone, 0, nil
		}

		// If the line is JSONL, it should marshall into map[string]interface{}
		var vals map[string]interface{}
		err = stdj.Unmarshal(line, &vals)
		if err != nil {
			return source.TypeNone, 0, nil
		}

		validLines++
		if validLines >= driver.Tuning.SampleSize {
			break
		}
	}

	if err = sc.Err(); err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}

	if validLines > 0 {
		return TypeJSONL, 1.0, nil
	}

	return source.TypeNone, 0, nil
}

func importJSONL(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, dbase driver.Database) error {
	r, err := openFn()
	if err != nil {
		return err
	}
	defer log.WarnIfCloseError(r)

	drvr := dbase.SQLDriver()
	db, err := dbase.DB().Conn(ctx)
	if err != nil {
		return errz.Err(err)
	}
	defer log.WarnIfCloseError(db)

	proc := newProcessor(true)
	sc := newLineScanner(ctx, r, '{')

	var hasMore bool
	var dirtySchema bool
	var line []byte
	var curSchema *importSchema
	var insertions []*insertion

	for {
		hasMore, line, err = sc.Next()
		if err != nil {
			return err
		}

		if dirtySchema {
			if !hasMore || sc.validLineCount > driver.Tuning.SampleSize {
				log.Debugf("line[%d]: time to (re)build the schema", sc.totalLineCount)
				if curSchema == nil {
					log.Debug("First time building the schema")
				}

				var newSchema *importSchema
				newSchema, err = proc.buildSchemaFlat()
				if err != nil {
					return err
				}

				err = execSchemaDelta(ctx, log, drvr, db, curSchema, newSchema)
				if err != nil {
					return err
				}

				curSchema = newSchema
				newSchema = nil

				insertions, err = proc.buildInsertionsFlat(curSchema)
				if err != nil {
					return err
				}

				err = execInsertions(ctx, log, drvr, db, insertions)
				if err != nil {
					return err
				}
			}

			if !hasMore {
				break
			}
		}

		var m map[string]interface{}
		dec := stdj.NewDecoder(bytes.NewReader(line))

		err = dec.Decode(&m)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return errz.Err(err)
		}

		dirtySchema, err = proc.processObject(m)
		if err != nil {
			return err
		}

		// If there's already a schema (curSchema != nil), then we
		// want to immediately insert new rows from the processor.
		// However, if the schema is dirty, wait for the top of the
		// loop (where the schema will be rebuilt) before insertion.
		if curSchema != nil && !dirtySchema {

		}

		// FIXME: need to add values that are created after schema creation
	}

	if sc.validLineCount == 0 {
		return errz.New("empty JSONL input")
	}

	return nil
}

// lineScanner scans lines of JSON. Empty lines are skipped. Thus
// totalLineCount may be greater than validLineCount. If a non-empty
// line does not begin with requireAnchor, an error is returned.
type lineScanner struct {
	ctx            context.Context
	sc             *bufio.Scanner
	requireAnchor  byte
	totalLineCount int
	validLineCount int
}

func newLineScanner(ctx context.Context, r io.Reader, requireAnchor byte) *lineScanner {
	return &lineScanner{ctx: ctx, sc: bufio.NewScanner(r), requireAnchor: requireAnchor}
}

// Next returns the next non-empty line.
func (ls *lineScanner) Next() (hasMore bool, line []byte, err error) {
	for {
		select {
		case <-ls.ctx.Done():
			return false, nil, ls.ctx.Err()
		default:
		}

		hasMore = ls.sc.Scan()
		if !hasMore {
			return false, nil, nil
		}

		if err = ls.sc.Err(); err != nil {
			return false, nil, errz.Err(err)
		}

		line = ls.sc.Bytes()
		ls.totalLineCount++
		if len(line) == 0 {
			// Probably want to skip blank lines? Maybe
			continue
		}

		if line[0] != ls.requireAnchor {
			return false, nil, errz.Errorf("line %d expected to begin with '%v' but got '%v'",
				ls.totalLineCount-1, rune(ls.requireAnchor), rune(line[0]))
		}

		ls.validLineCount++
		return true, line, nil
	}
}
