package csv

import (
	"context"
	"encoding/csv"
	"io"
	"strconv"
	"unicode/utf8"

	"github.com/neilotoole/lg"
	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/sqlmodel"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const (
	readAheadBufferSize = 100
)

// importCSV loads the src CSV data to scratchDB.
func importCSV(ctx context.Context, log lg.Log, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	// TODO: optPredictKind should be read from src.Options.
	const optPredictKind bool = true

	var err error
	var r io.ReadCloser

	r, err = openFn()
	if err != nil {
		return err
	}

	defer log.WarnIfCloseError(r)

	// We add the CR filter reader to deal with CSV files exported
	// from Excel which can have the DOS-style \r EOL markers.
	cr := csv.NewReader(&crFilterReader{r: r})
	cr.Comma, err = getDelimiter(src)
	if err != nil {
		return err
	}

	// readAheadRecs temporarily holds records read from r for the purpose
	// of determining CSV metadata such as column headers, data kind etc.
	// These records will later be written to recordCh.
	readAheadRecs := make([][]string, 0, readAheadBufferSize)

	colNames, err := getColNames(cr, src, &readAheadRecs)
	if err != nil {
		return err
	}

	var expectFieldCount = len(colNames)

	var colKinds []kind.Kind
	if optPredictKind {
		colKinds, err = predictColKinds(expectFieldCount, cr, &readAheadRecs, readAheadBufferSize)
		if err != nil {
			return err
		}
	} else {
		// If we're not predicting col kind, then we use kind.Text.
		colKinds = make([]kind.Kind, expectFieldCount)
		for i := range colKinds {
			colKinds[i] = kind.Text
		}
	}

	// And now we need to create the dest table in scratchDB
	tblDef := createTblDef(source.MonotableName, colNames, colKinds)

	err = scratchDB.SQLDriver().CreateTable(ctx, scratchDB.DB(), tblDef)
	if err != nil {
		return errz.Wrap(err, "csv: failed to create dest scratch table")
	}

	recMeta, err := getRecMeta(ctx, scratchDB, tblDef)
	if err != nil {
		return err
	}

	insertWriter := libsq.NewDBWriter(log, scratchDB, tblDef.Name, driver.Tuning.RecordChSize)
	err = execInsert(ctx, insertWriter, recMeta, readAheadRecs, cr)
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

// execInsert inserts the CSV records in readAheadRecs (followed by records
// from the csv.Reader) via recw. The caller should wait on recw to complete.
func execInsert(ctx context.Context, recw libsq.RecordWriter, recMeta sqlz.RecordMeta, readAheadRecs [][]string, r *csv.Reader) error {
	ctx, cancelFn := context.WithCancel(ctx)

	recordCh, errCh, err := recw.Open(ctx, cancelFn, recMeta)
	if err != nil {
		return err
	}
	defer close(recordCh)

	// Before we continue reading from CSV, we first write out
	// any CSV records we read earlier.
	for i := range readAheadRecs {
		rec := mungeCSV2InsertRecord(readAheadRecs[i])

		select {
		case err = <-errCh:
			cancelFn()
			return err
		case <-ctx.Done():
			cancelFn()
			return ctx.Err()
		case recordCh <- rec:
		}
	}

	var csvRecord []string
	for {
		csvRecord, err = r.Read()
		if err == io.EOF {
			// We're done reading
			return nil
		}
		if err != nil {
			cancelFn()
			return errz.Wrap(err, "read from CSV data source")
		}

		rec := mungeCSV2InsertRecord(csvRecord)

		select {
		case err = <-errCh:
			cancelFn()
			return err
		case <-ctx.Done():
			cancelFn()
			return ctx.Err()
		case recordCh <- rec:
		}
	}
}

// mungeCSV2InsertRecord returns a new []interface{} containing
// the values of the csvRec []string.
func mungeCSV2InsertRecord(csvRec []string) []interface{} {
	a := make([]interface{}, len(csvRec))
	for i := range csvRec {
		a[i] = csvRec[i]
	}
	return a
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

func createTblDef(tblName string, colNames []string, kinds []kind.Kind) *sqlmodel.TableDef {
	tbl := &sqlmodel.TableDef{Name: tblName}

	cols := make([]*sqlmodel.ColDef, len(colNames))
	for i := range colNames {
		cols[i] = &sqlmodel.ColDef{Table: tbl, Name: colNames[i], Kind: kinds[i]}
	}

	tbl.Cols = cols
	return tbl
}

// predictColKinds examines up to maxExamine records in readAheadRecs
// and those returned by r to guess the kind of each field.
// Any additional records read from r are appended to readAheadRecs.
//
// This func considers these candidate kinds, in order of
// precedence: kind.Int, kind.Bool, kind.Decimal.
//
// kind.Decimal is chosen over kind.Float due to its greater flexibility.
// NOTE: Currently kind.Time and kind.Datetime are not examined.
//
// If any field (string) value cannot be parsed into a particular kind, that
// kind is excluded from the list of candidate kinds. The first of any
// remaining candidate kinds for each field is returned, or kind.Text if
// no candidate kinds.
func predictColKinds(expectFieldCount int, r *csv.Reader, readAheadRecs *[][]string, maxExamine int) ([]kind.Kind, error) {
	// FIXME: [legacy] this function should switch to using kind.Detector

	candidateKinds := newCandidateFieldKinds(expectFieldCount)
	var examineCount int

	// First, read any records from the readAheadRecs buffer
	for recIndex := 0; recIndex < len(*readAheadRecs) && examineCount < maxExamine; recIndex++ {
		for fieldIndex := 0; fieldIndex < expectFieldCount; fieldIndex++ {
			candidateKinds[fieldIndex] = excludeFieldKinds(candidateKinds[fieldIndex], (*readAheadRecs)[recIndex][fieldIndex])
		}
		examineCount++
	}

	// Next, continue to read from r until we reach maxExamine records.
	for ; examineCount < maxExamine; examineCount++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errz.Err(err)
		}

		if len(rec) != expectFieldCount {
			// safety check
			return nil, errz.Errorf("expected %d fields in CSV record but got %d", examineCount, len(rec))
		}

		for fieldIndex, fieldValue := range rec {
			candidateKinds[fieldIndex] = excludeFieldKinds(candidateKinds[fieldIndex], fieldValue)
		}

		// Add the recently read record to readAheadRecs so that
		// it's not lost.
		*readAheadRecs = append(*readAheadRecs, rec)
	}

	resultKinds := make([]kind.Kind, expectFieldCount)
	for i := range resultKinds {
		switch len(candidateKinds[i]) {
		case 0:
			// If all candidate kinds have been excluded, kind.Text is
			// the fallback option.
			resultKinds[i] = kind.Text
		default:
			// If there's one or more candidate kinds remaining, pick the first
			// one available, as it should be the most specific kind.
			resultKinds[i] = candidateKinds[i][0]
		}
	}
	return resultKinds, nil
}

// newCandidateFieldKinds returns a new slice of kind.Kind containing
// potential kinds for a field/column. The kinds are in an order of
// precedence.
func newCandidateFieldKinds(n int) [][]kind.Kind {
	kinds := make([][]kind.Kind, n)
	for i := range kinds {
		k := []kind.Kind{
			kind.Int,
			kind.Bool,
			kind.Decimal,
		}
		kinds[i] = k
	}

	return kinds
}

// excludeFieldKinds returns a filter of fieldCandidateKinds, removing those
// kinds which fieldVal cannot be converted to.
func excludeFieldKinds(fieldCandidateKinds []kind.Kind, fieldVal string) []kind.Kind {
	var resultCandidateKinds []kind.Kind

	if fieldVal == "" {
		// If the field is empty string, this could indicate a NULL value
		// for any kind. That is, we don't exclude a candidate kind due
		// to empty string.
		return fieldCandidateKinds
	}

	for _, knd := range fieldCandidateKinds {
		var err error

		switch knd {
		case kind.Int:
			_, err = strconv.Atoi(fieldVal)

		case kind.Bool:
			_, err = strconv.ParseBool(fieldVal)

		case kind.Decimal:
			_, err = decimal.NewFromString(fieldVal)
		default:
		}

		if err == nil {
			resultCandidateKinds = append(resultCandidateKinds, knd)
		}
	}

	return resultCandidateKinds
}

// getColNames determines column names. The col names are determined
// as follows:
//
//   - Col names can be explicitly specified in src.Options
//   - If the source CSV has a header record, the fields of the
//     header record are returned.
//   - Otherwise, the first data record is read, and col names are generated
//     based on the number of fields [A,B,C...] etc. That first data record
//     is appended to readAheadRecs so that it's not lost.
//
// Note that cr must not have been previously read.
func getColNames(cr *csv.Reader, src *source.Source, readAheadRecs *[][]string) ([]string, error) {
	// If col names are explicitly provided in opts, we
	// will be returning them.
	explicitColNames, err := options.GetColNames(src.Options)
	if err != nil {
		return nil, err
	}

	optHasHeaderRecord, _, err := options.HasHeader(src.Options)
	if err != nil {
		return nil, err
	}

	if optHasHeaderRecord {
		// The CSV file has a header record, we need to consume it.
		var headerRec []string
		headerRec, err = cr.Read()
		if err == io.EOF {
			return nil, errz.Errorf("data source %s has no data", src.Handle)
		}
		if err != nil {
			return nil, errz.Err(err)
		}

		if len(explicitColNames) > 0 {
			// If col names were explicitly specified via options, return
			// those col names, as explicit option col names have precedence
			// over the header record col names.
			return explicitColNames, nil
		}

		// Otherwise return the header record col names.
		return headerRec, nil
	}

	// The CSV file does not have a header record. We will generate
	// col names [A,B,C...]. To do so, we need to know how many fields
	// there are in the first record.
	firstDataRecord, err := cr.Read()
	if err == io.EOF {
		return nil, errz.Errorf("data source %s is empty", src.Handle)
	}
	if err != nil {
		return nil, errz.Wrapf(err, "read from data source %s", src.Handle)
	}

	// firstRecord contains actual data, so append it to initialRecs.
	*readAheadRecs = append(*readAheadRecs, firstDataRecord)

	// If no column names yet, we generate them based on the number
	// of fields in firstDataRecord.
	generatedColNames := make([]string, len(firstDataRecord))
	for i := range firstDataRecord {
		generatedColNames[i] = stringz.GenerateAlphaColName(i, false)
	}

	return generatedColNames, nil
}

// namedDelimiters is map of named delimiter strings to
// rune value. For example, "comma" maps to ',' and "pipe" maps to '|'.
var namedDelimiters = map[string]rune{
	"comma":  ',',
	"space":  ' ',
	"pipe":   '|',
	"tab":    '\t',
	"colon":  ':',
	"semi":   ';',
	"period": '.',
}

// getDelimiter returns the delimiter for src. An explicit
// delimiter value may be set in src.Options; otherwise
// the default for the source is returned.
func getDelimiter(src *source.Source) (rune, error) {
	delim, ok, err := getDelimFromOptions(src.Options)
	if err != nil {
		return 0, err
	}

	if ok {
		return delim, nil
	}

	if src.Type == TypeTSV {
		return '\t', nil
	}

	// default is comma
	return ',', nil
}

// getDelimFromOptions returns ok as true and the delimiter rune if a
// valid value is provided in src.Options, returns ok as false if
// no valid value provided, and an error if the provided value is invalid.
func getDelimFromOptions(opts options.Options) (r rune, ok bool, err error) {
	if len(opts) == 0 {
		return 0, false, nil
	}

	const key = "delim"
	_, ok = opts[key]
	if !ok {
		return 0, false, nil
	}

	val := opts.Get(key)
	if val == "" {
		return 0, false, nil
	}

	if len(val) == 1 {
		r, _ = utf8.DecodeRuneInString(val)
		return r, true, nil
	}

	r, ok = namedDelimiters[val]

	if !ok {
		err = errz.Errorf("unknown delimiter constant %q", val)
		return 0, false, err
	}

	return r, true, nil
}

// crFilterReader is a reader whose Read method converts
// standalone carriage return '\r' bytes to newline '\n'.
// CRLF "\r\n" sequences are untouched.
// This is useful for reading from DOS format, e.g. a TSV
// file exported by Microsoft Excel.
type crFilterReader struct {
	r io.Reader
}

func (r *crFilterReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)

	for i := 0; i < n; i++ {
		if p[i] == 13 {
			if i+1 < n && p[i+1] == 10 {
				continue // it's \r\n
			}
			// it's just \r by itself, replace
			p[i] = 10
		}
	}

	return n, err
}
