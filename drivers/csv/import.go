package csv

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"unicode/utf8"

	"github.com/neilotoole/sq/libsq/core/stringz"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

const sampleSize = 100

// importCSV loads the src CSV data into scratchDB.
func importCSV(ctx context.Context, src *source.Source, openFn source.FileOpenFunc, scratchDB driver.Database) error {
	log := lg.FromContext(ctx)

	var err error
	var r io.ReadCloser

	r, err = openFn()
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	delim, err := getDelimiter(src)
	if err != nil {
		return err
	}

	cr := newCSVReader(r, delim)
	recs, err := readRecords(cr, sampleSize)
	if err != nil {
		return err
	}

	headerPresent, err := hasHeaderRow(recs, src.Options)
	if err != nil {
		return err
	}

	var header []string
	if headerPresent {
		header = recs[0]

		// We're done with the first row
		recs = recs[1:]
	} else {
		// The CSV file does not have a header record. We will generate
		// col names [A,B,C...].
		header = make([]string, len(recs[0]))
		for i := range recs[0] {
			header[i] = stringz.GenerateAlphaColName(i, false)
		}
	}

	kinds, _, err := detectColKinds(recs)
	if err != nil {
		return err
	}

	// And now we need to create the dest table in scratchDB
	tblDef := createTblDef(source.MonotableName, header, kinds)

	err = scratchDB.SQLDriver().CreateTable(ctx, scratchDB.DB(), tblDef)
	if err != nil {
		return errz.Wrap(err, "csv: failed to create dest scratch table")
	}

	recMeta, err := getRecMeta(ctx, scratchDB, tblDef)
	if err != nil {
		return err
	}

	insertWriter := libsq.NewDBWriter(log, scratchDB, tblDef.Name, driver.Tuning.RecordChSize)
	err = execInsert(ctx, insertWriter, recMeta, recs, cr)
	if err != nil {
		return err
	}

	inserted, err := insertWriter.Wait()
	if err != nil {
		return err
	}

	log.Debug("Inserted rows",
		lga.Count, inserted,
		lga.Target, source.Target(scratchDB.Source(), tblDef.Name),
	)
	return nil
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

	_, ok = opts[options.OptDelim]
	if !ok {
		return 0, false, nil
	}

	val := opts.Get(options.OptDelim)
	if val == "" {
		return 0, false, nil
	}

	if len(val) == 1 {
		r, _ = utf8.DecodeRuneInString(val)
		return r, true, nil
	}

	r, ok = namedDelimiters[val]

	if !ok {
		err = errz.Errorf("unknown delimiter constant {%s}", val)
		return 0, false, err
	}

	return r, true, nil
}

func newCSVReader(r io.Reader, delim rune) *csv.Reader {
	// We add the CR filter reader to deal with CSV files exported
	// from Excel which can have the DOS-style \r EOL markers.
	cr := csv.NewReader(&crFilterReader{r: r})
	cr.Comma = delim
	return cr
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

// readRecords reads a maximum of n records from cr.
func readRecords(cr *csv.Reader, n int) ([][]string, error) {
	recs := make([][]string, 0, n)

	for i := 0; i < n; i++ {
		rec, err := cr.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return recs, nil
			}

			return nil, errz.Err(err)
		}
		recs = append(recs, rec)
	}

	return recs, nil
}
