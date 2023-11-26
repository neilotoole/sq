package csv

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"time"
	"unicode/utf8"

	"github.com/neilotoole/sq/libsq"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// OptEmptyAsNull determines if an empty CSV field is treated as NULL
// or as the zero value for the kind of that field.
var OptEmptyAsNull = options.NewBool(
	"driver.csv.empty-as-null",
	"",
	0,
	true,
	"Treat ingest empty CSV fields as NULL",
	`When true, empty CSV fields are treated as NULL. When false,
the zero value for that type is used, e.g. empty string or 0.`,
	options.TagSource,
	"csv",
)

// OptDelim specifies the CSV delimiter to use.
var OptDelim = options.NewString(
	"driver.csv.delim",
	"",
	0,
	delimCommaKey,
	nil,
	"Delimiter for ingest CSV data",
	`Delimiter to use for CSV files. Default is "comma".
Possible values are: comma, space, pipe, tab, colon, semi, period.`,
	options.TagSource,
	"csv",
)

// ingestCSV loads the src CSV data into scratchDB.
func ingestCSV(ctx context.Context, src *source.Source, openFn source.FileOpenFunc, scratchPool driver.Pool) error {
	log := lg.FromContext(ctx)
	startUTC := time.Now().UTC()

	var err error
	var r io.ReadCloser

	r, err = openFn(ctx)
	if err != nil {
		return err
	}
	defer lg.WarnIfCloseError(log, lgm.CloseFileReader, r)

	delim, err := getDelimiter(src)
	if err != nil {
		return err
	}

	cr := newCSVReader(r, delim)
	recs, err := readRecords(cr, driver.OptIngestSampleSize.Get(src.Options))
	if err != nil {
		return err
	}

	headerPresent, err := hasHeaderRow(ctx, recs, src.Options)
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

	if header, err = driver.MungeIngestColNames(ctx, header); err != nil {
		return err
	}

	kinds, mungers, err := detectColKinds(recs)
	if err != nil {
		return err
	}

	// And now we need to create the dest table in scratchDB
	tblDef := createTblDef(source.MonotableName, header, kinds)

	db, err := scratchPool.DB(ctx)
	if err != nil {
		return err
	}

	err = scratchPool.SQLDriver().CreateTable(ctx, db, tblDef)
	if err != nil {
		return errz.Wrap(err, "csv: failed to create dest scratch table")
	}

	recMeta, err := getIngestRecMeta(ctx, scratchPool, tblDef)
	if err != nil {
		return err
	}

	if OptEmptyAsNull.Get(src.Options) {
		configureEmptyNullMunge(mungers, recMeta)
	}

	insertWriter := libsq.NewDBWriter(
		scratchPool,
		tblDef.Name,
		driver.OptTuningRecChanSize.Get(scratchPool.Source().Options),
	)
	err = execInsert(ctx, insertWriter, recMeta, mungers, recs, cr)
	if err != nil {
		return err
	}

	inserted, err := insertWriter.Wait()
	if err != nil {
		return err
	}

	log.Debug("Inserted rows",
		lga.Count, inserted,
		lga.Elapsed, time.Since(startUTC).Round(time.Millisecond),
		lga.Target, source.Target(scratchPool.Source(), tblDef.Name),
	)
	return nil
}

// configureEmptyNullMunge configures mungers to that empty string is
// munged to nil.
func configureEmptyNullMunge(mungers []kind.MungeFunc, recMeta record.Meta) {
	kinds := recMeta.Kinds()
	for i := range mungers {
		if kinds[i] == kind.Text {
			if mungers[i] == nil {
				mungers[i] = kind.MungeEmptyStringAsNil
				continue
			}

			// There's already a munger: wrap it
			existing := mungers[i]
			mungers[i] = func(v any) (any, error) {
				var err error
				v, err = existing(v)
				if err != nil {
					return v, err
				}

				return kind.MungeEmptyStringAsNil(v)
			}
		}
	}
}

const (
	delimCommaKey  = "comma"
	delimComma     = ','
	delimSpaceKey  = "space"
	delimSpace     = ' '
	delimPipeKey   = "pipe"
	delimPipe      = '|'
	delimTabKey    = "tab"
	delimTab       = '\t'
	delimColonKey  = "colon"
	delimColon     = ':'
	delimSemiKey   = "semi"
	delimSemi      = ';'
	delimPeriodKey = "period"
	delimPeriod    = '.'
)

// NamedDelims returns the named delimiters, such as [comma, tab, pipe...].
func NamedDelims() []string {
	return []string{
		delimCommaKey,
		delimTabKey,
		delimSemiKey,
		delimColonKey,
		delimSpaceKey,
		delimPipeKey,
		delimPeriodKey,
	}
}

// namedDelimiters is map of named delimiter strings to
// rune value. For example, "comma" maps to ',' and "pipe" maps to '|'.
var namedDelimiters = map[string]rune{
	delimCommaKey:  delimComma,
	delimSpaceKey:  delimSpace,
	delimPipeKey:   delimPipe,
	delimTabKey:    delimTab,
	delimColonKey:  delimColon,
	delimSemiKey:   delimSemi,
	delimPeriodKey: delimPeriod,
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

	if !OptDelim.IsSet(opts) {
		return 0, false, nil
	}

	val := OptDelim.Get(opts)
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
