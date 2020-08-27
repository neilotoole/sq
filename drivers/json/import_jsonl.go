package json

import (
	"bufio"
	"bytes"
	"context"
	stdj "encoding/json"
	"fmt"
	"io"
	"math"

	"github.com/neilotoole/lg"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// detectColKindsJSONL reads JSONL lines from r, and returns
// the kind of each field. The []readMungeFunc may contain a munge
// func that should be applied to each value (or the element may be nil).
func detectColKindsJSONL(ctx context.Context, r io.Reader) (names []string, kinds []kind.Kind, mungeFns []kind.MungeFunc, err error) {
	var (
		totalLineCount int
		// jLineCount is the number of JSONL lines (totalLineCount minus empty lines)
		jLineCount int
		line       []byte
		detectors  []*kind.Detector
	)

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return nil, nil, nil, ctx.Err()
		default:
		}

		if jLineCount > driver.Tuning.SampleSize {
			break
		}

		if err = sc.Err(); err != nil {
			return nil, nil, nil, errz.Err(err)
		}

		line = sc.Bytes()
		totalLineCount++
		if len(line) == 0 {
			// Probably want to skip blank lines? Maybe
			continue
		}

		jLineCount++

		// Each line of JSONL must open with left brace
		if line[0] != '{' {
			return nil, nil, nil, errz.New("line does not begin with left bracket '['")
		}

		// If the line is JSONL it should marshall into map[string]interface{}
		var obj map[string]interface{}
		err = stdj.Unmarshal(line, &obj)
		if err != nil {
			return nil, nil, nil, errz.Err(err)
		}

		if len(obj) == 0 {
			return nil, nil, nil, errz.Errorf("zero field count at line %d", totalLineCount)
		}

		if kinds == nil {
			kinds = make([]kind.Kind, len(obj))
			mungeFns = make([]kind.MungeFunc, len(obj))
			detectors = make([]*kind.Detector, len(obj))
			for i := range detectors {
				detectors[i] = kind.NewDetector()
			}
		}

		if len(obj) != len(kinds) {
			return nil, nil, nil, errz.Errorf("inconsistent field count: expected %d but got %d at line %d",
				len(kinds), len(obj), totalLineCount)
		}

		var j int
		for _, val := range obj {
			// Special case: The decoder decodes numbers into float.
			// Which we don't want, if the number is really an int
			// (especially important for id columns).
			// So, if the float has zero after the decimal point '.' (that
			// is to say, it's a round float like 1.0), we convert the float
			// to an int. Possibly we could use json.Decoder.UseNumber to
			// avoid this, but that may introduce other complexities.
			fVal, ok := val.(float64)
			if ok {
				floor := math.Floor(fVal)
				if fVal-floor == 0 {
					val = int64(floor)
				}
			}

			detectors[j].Sample(val)
			j++
		}
	}

	if jLineCount == 0 {
		return nil, nil, nil, errz.New("empty JSONA input")
	}

	for i := range kinds {
		kinds[i], mungeFns[i], err = detectors[i].Detect()
		if err != nil {
			return nil, nil, nil, err
		}
	}

	return names, kinds, mungeFns, nil
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

				log.Debugf("Creating new schema: %#v", *newSchema)
				err = execSchemaDelta(ctx, log, drvr, db, curSchema, newSchema)
				if err != nil {
					return err
				}
				fmt.Printf("created new schema")
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
		//dec.UseNumber()

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

		//if dirtySchema {
		//	proc.clearDirty()
		//	fmt.Printf("line [%d]: schema dirtied\n", sc.totalLineCount-1)
		//}

	}

	if sc.validLineCount == 0 {
		return errz.New("empty JSONL input")
	}

	//schema, err := proc.buildSchemaFlat()
	//if err != nil {
	//	return err
	//}
	//
	//q.Q(schema)

	return nil
}

type lineScanner struct {
	ctx            context.Context
	sc             *bufio.Scanner
	matchAnchor    byte
	totalLineCount int
	validLineCount int
}

func newLineScanner(ctx context.Context, r io.Reader, anchor byte) *lineScanner {
	return &lineScanner{ctx: ctx, sc: bufio.NewScanner(r), matchAnchor: anchor}
}

// Next returns the next non-empty line.
func (ls *lineScanner) Next() (ok bool, line []byte, err error) {
	for {
		select {
		case <-ls.ctx.Done():
			return false, nil, ls.ctx.Err()
		default:
		}

		ok = ls.sc.Scan()
		if !ok {
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

		if line[0] != ls.matchAnchor {
			return false, nil, errz.Errorf("line %d did not start with '%v' but started with '%v'",
				ls.totalLineCount-1, rune(ls.matchAnchor), rune(line[0]))
		}

		ls.validLineCount++
		return true, line, nil
	}

	return false, nil, nil
}
