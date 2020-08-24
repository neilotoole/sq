package json

import (
	"bufio"
	"context"
	stdj "encoding/json"
	"io"
	"math"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

type importFunc func(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error

var (
	_ importFunc = importJSON
	_ importFunc = importJSONA
	_ importFunc = importJSONL
)

func importJSON(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	log.Warn("not implemented")
	return nil
}

func importJSONA(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	predictR, err := openFn()
	if err != nil {
		return errz.Err(err)
	}

	defer log.WarnIfCloseError(predictR)

	colKinds, readMungeFns, err := predictColKindsJSONA(ctx, predictR)
	if err != nil {
		return err
	}

	if len(colKinds) == 0 || len(readMungeFns) == 0 {
		return errz.Errorf("import %s: number of fields is zero", TypeJSONA)
	}

	colNames := make([]string, len(colKinds))
	for i := 0; i < len(colNames); i++ {
		colNames[i] = stringz.GenerateAlphaColName(i, true)
	}

	// And now we need to create the dest table in scratchDB
	tblDef := sqlmodel.NewTableDef(source.MonotableName, colNames, colKinds)
	err = scratchDB.SQLDriver().CreateTable(ctx, scratchDB.DB(), tblDef)
	if err != nil {
		return errz.Wrapf(err, "import %s: failed to create dest scratch table", TypeJSONA)
	}

	recMeta, err := getRecMeta(ctx, scratchDB, tblDef)
	if err != nil {
		return err
	}

	const insertChSize = 100

	r, err := openFn()
	if err != nil {
		return errz.Err(err)
	}
	defer log.WarnIfCloseError(r)

	insertWriter := libsq.NewDBWriter(log, scratchDB, tblDef.Name, insertChSize)

	var cancelFn context.CancelFunc
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()

	recordCh, errCh, err := insertWriter.Open(ctx, cancelFn, recMeta)
	if err != nil {
		return err
	}

	err = startInsertJSONA(ctx, recordCh, errCh, r, readMungeFns)
	if err != nil {
		return err
	}

	inserted, err := insertWriter.Wait()
	if err != nil {
		return err
	}

	log.Debugf("Inserted %d rows to %s.%s", inserted, scratchDB.Source().Handle, tblDef.Name)
	return nil
}

// startInsertJSONA reads JSON records from r and sends
// them on recordCh.
func startInsertJSONA(ctx context.Context, recordCh chan<- sqlz.Record, errCh <-chan error, r io.Reader, mungeFns []func(interface{}) (interface{}, error)) error {
	defer close(recordCh)

	sc := bufio.NewScanner(r)
	var line []byte
	var err error

	for sc.Scan() {
		if err = sc.Err(); err != nil {
			return errz.Err(err)
		}

		line = sc.Bytes()
		if len(line) == 0 {
			// Probably want to skip blank lines? Maybe
			continue
		}

		// Each line of JSONA must open with left bracket
		if line[0] != '[' {
			return errz.New("malformed JSONA input")
		}

		// If the line is JSONA, it should marshal into []interface{}
		var rec []interface{}
		err = stdj.Unmarshal(line, &rec)
		if err != nil {
			return errz.Err(err)
		}

		for i := 0; i < len(rec); i++ {
			fn := mungeFns[i]
			if fn != nil {
				v, err := fn(rec[i])
				if err != nil {
					return errz.Err(err)
				}
				rec[i] = v
			}
		}

		select {
		case err = <-errCh:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case recordCh <- rec:
		}
	}

	if err = sc.Err(); err != nil {
		return errz.Err(err)
	}

	return nil
}

func predictColKindsJSONA(ctx context.Context, r io.Reader) ([]kind.Kind, []func(interface{}) (interface{}, error), error) {
	var (
		err            error
		totalLineCount int
		// jLineCount is the number of JSONA lines (totalLineCount minus empty lines)
		jLineCount int
		line       []byte
		kinds      []kind.Kind
		detectors  []*kind.Detector
		mungeFns   []func(interface{}) (interface{}, error)
	)

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		if jLineCount > sampleSize {
			break
		}

		if err = sc.Err(); err != nil {
			return nil, nil, errz.Err(err)
		}

		line = sc.Bytes()
		totalLineCount++
		if len(line) == 0 {
			// Probably want to skip blank lines? Maybe
			continue
		}

		jLineCount++

		// Each line of JSONA must open with left bracket
		if line[0] != '[' {
			return nil, nil, errz.New("line does not begin with left bracket '['")
		}

		// If the line is JSONA, it should marshall into []interface{}
		var vals []interface{}
		err = stdj.Unmarshal(line, &vals)
		if err != nil {
			return nil, nil, errz.Err(err)
		}

		if len(vals) == 0 {
			return nil, nil, errz.Errorf("zero field count at line %d", totalLineCount)
		}

		if kinds == nil {
			kinds = make([]kind.Kind, len(vals))
			mungeFns = make([]func(interface{}) (interface{}, error), len(vals))
			detectors = make([]*kind.Detector, len(vals))
			for i := range detectors {
				detectors[i] = kind.NewDetector()
			}
		}

		if len(vals) != len(kinds) {
			return nil, nil, errz.Errorf("inconsistent field count: expected %d but got %d at line %d",
				len(kinds), len(vals), totalLineCount)
		}

		//
		for i, val := range vals {
			// Special case: The decoder can decode an int into a float.
			// If the float has zero after the decimal point '.' (that
			// is to say, it's a round float like 1.0), we convert the float
			// to an int
			fVal, ok := val.(float64)
			if ok {
				floor := math.Floor(fVal)
				if fVal-floor == 0 {
					val = int64(floor)
				}
			}

			detectors[i].Sample(val)
		}
	}

	if jLineCount == 0 {
		return nil, nil, errz.New("empty JSONA input")
	}

	for i := range kinds {
		kinds[i], mungeFns[i], err = detectors[i].Detect()
		if err != nil {
			return nil, nil, err
		}
	}

	return kinds, mungeFns, nil
}

func importJSONL(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	return errz.New("not implemented")
}

// getRecMeta returns RecordMeta to use with RecordWriter.Open.
func getRecMeta(ctx context.Context, scratchDB driver.Database, tblDef *sqlmodel.TableDef) (sqlz.RecordMeta, error) {
	colTypes, err := scratchDB.SQLDriver().TableColumnTypes(ctx, scratchDB.DB(), tblDef.Name, tblDef.ColNames())
	if err != nil {
		return nil, err
	}

	destMeta, _, err := scratchDB.SQLDriver().RecordMeta(colTypes)
	if err != nil {
		return nil, err
	}

	return destMeta, nil
}
