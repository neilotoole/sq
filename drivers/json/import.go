package json

import (
	"bufio"
	"context"
	stdj "encoding/json"
	"io"
	"math"

	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/sqlz"
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
	log.Warn("not implemented")
	return nil
}

func predictColKindsJSONA(ctx context.Context, r io.Reader) ([]sqlz.Kind, error) {
	var (
		err            error
		totalLineCount int
		// jLineCount is the number of JSONA lines (totalLineCount minus empty lines)
		jLineCount int
		line       []byte
		kinds      []sqlz.Kind
		detectors  []*sqlz.KindDetector
	)

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if jLineCount > sampleSize {
			break
		}

		if err = sc.Err(); err != nil {
			return nil, errz.Err(err)
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
			return nil, errz.New("line does not begin with left bracket '['")
		}

		// If the line is JSONA, it should marshall into []interface{}
		var vals []interface{}
		err = stdj.Unmarshal(line, &vals)
		if err != nil {
			return nil, errz.Err(err)
		}

		if len(vals) == 0 {
			return nil, errz.Errorf("zero field count at line %d", totalLineCount)
		}

		if kinds == nil {
			kinds = make([]sqlz.Kind, len(vals))
			detectors = make([]*sqlz.KindDetector, len(vals))
			for i := range detectors {
				detectors[i] = sqlz.NewKindDetector()
			}
		}

		if len(vals) != len(kinds) {
			return nil, errz.Errorf("inconsistent field count: expected %d but got %d at line %d",
				len(kinds), len(vals), totalLineCount)
		}

		for i, val := range vals {
			// Special case: The decoder can decode an int into a float.
			// If the float has a zero after the decimal point '.' (that
			// is to say, it's really a round int), we convert the float
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
		return nil, errz.New("empty JSONA input")
	}

	for i := range kinds {
		kinds[i], _, err = detectors[i].Detect() // FIXME: deal with the mungeFn
		if err != nil {
			return nil, err
		}
	}

	return kinds, nil
}

func importJSONL(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	return errz.New("not implemented")

	//const optPredictKind bool = true
	//
	//var err error
	//var r io.ReadCloser
	//
	//r, err = openFn()
	//if err != nil {
	//	return err
	//}
	//
	//// We add the CR filter reader to deal with CSV files exported
	//// from Excel which can have the DOS-style \r EOL markers.
	//cr := csv.NewReader(&crFilterReader{r: r})
	//cr.Comma, err = getDelimiter(src)
	//if err != nil {
	//	return err
	//}
	//
	//// readAheadRecs temporarily holds records read from r for the purpose
	//// of determining CSV metadata such as column headers, data kind etc.
	//// These records will later be written to recordCh.
	//readAheadRecs := make([][]string, 0, readAheadBufferSize)
	//
	//colNames, err := getColNames(cr, src, &readAheadRecs)
	//if err != nil {
	//	return err
	//}
	//
	//var expectFieldCount = len(colNames)
	//
	//var colKinds []sqlz.Kind
	//if optPredictKind {
	//	colKinds, err = predictColKinds(expectFieldCount, cr, &readAheadRecs, readAheadBufferSize)
	//	if err != nil {
	//		return err
	//	}
	//} else {
	//	// If we're not predicting col kind, then we use KindText.
	//	colKinds = make([]sqlz.Kind, expectFieldCount)
	//	for i := range colKinds {
	//		colKinds[i] = sqlz.KindText
	//	}
	//}
	//
	//// And now we need to create the dest table in scratchDB
	//tblDef := createTblDef(source.MonotableName, colNames, colKinds)
	//
	//err = scratchDB.SQLDriver().CreateTable(ctx, scratchDB.DB(), tblDef)
	//if err != nil {
	//	return core.errz.Wrap(err, "csv: failed to create dest scratch table")
	//}
	//
	//recMeta, err := getRecMeta(ctx, scratchDB, tblDef)
	//if err != nil {
	//	return err
	//}
	//
	//insertWriter := libsq.NewDBWriter(log, scratchDB, tblDef.Name, insertChSize)
	//err = execInsert(ctx, insertWriter, recMeta, readAheadRecs, cr)
	//if err != nil {
	//	return err
	//}
	//
	//inserted, err := insertWriter.Wait()
	//if err != nil {
	//	return err
	//}
	//
	//log.Debugf("Inserted %d rows to %s.%s", inserted, scratchDB.Source().Handle, tblDef.Name)
	//return nil
}
