package json

import (
	"bufio"
	"bytes"
	"context"
	stdj "encoding/json"
	"io"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// DetectJSONL implements source.DriverDetectFunc.
func DetectJSONL(ctx context.Context, openFn source.FileOpenFunc) (detected source.DriverType,
	score float32, err error,
) {
	log := lg.FromContext(ctx)
	var r io.ReadCloser
	r, err = openFn()
	if err != nil {
		return source.TypeNone, 0, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

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

		// If the line is JSONL, it should marshall into map[string]any
		var vals map[string]any
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

func importJSONL(ctx context.Context, job importJob) error { //nolint:gocognit
	log := lg.FromContext(ctx)

	r, err := job.openFn()
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	drvr := job.destDB.SQLDriver()
	db, err := job.destDB.DB().Conn(ctx)
	if err != nil {
		return errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDB, db)

	proc := newProcessor(job.flatten)
	scan := newLineScanner(ctx, r, '{')

	var (
		hasMore        bool
		schemaModified bool
		line           []byte
		curSchema      *importSchema
		insertions     []*insertion
	)

	for {
		hasMore, line, err = scan.next()
		if err != nil {
			return err
		}

		if schemaModified {
			if !hasMore || scan.validLineCount >= job.sampleSize {
				log.Debug("Time to (re)build the schema", lga.Line, scan.totalLineCount)
				if curSchema == nil {
					log.Debug("First time building the schema")
				}

				var newSchema *importSchema
				newSchema, err = proc.buildSchemaFlat()
				if err != nil {
					return err
				}

				err = execSchemaDelta(ctx, drvr, db, curSchema, newSchema)
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

				err = execInsertions(ctx, drvr, db, insertions)
				if err != nil {
					return err
				}
			}

			if !hasMore {
				// We're done
				break
			}
		}

		var m map[string]any
		dec := stdj.NewDecoder(bytes.NewReader(line))

		err = dec.Decode(&m)
		if err != nil {
			if err == io.EOF { //nolint:errorlint
				break
			}
			return errz.Err(err)
		}

		schemaModified, err = proc.processObject(m, line)
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

		err = execInsertions(ctx, drvr, db, insertions)
		if err != nil {
			return err
		}
	}

	if scan.validLineCount == 0 {
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

// next returns the next non-empty line.
func (ls *lineScanner) next() (hasMore bool, line []byte, err error) {
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
			return false, nil, errz.Errorf("line %d expected to begin with '%s' but got '%s'",
				ls.totalLineCount-1, string(ls.requireAnchor), string(line[0]))
		}

		ls.validLineCount++
		return true, line, nil
	}
}
