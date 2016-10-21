package csv

import (
	"strings"

	"sync"

	"encoding/csv"

	"fmt"
	"io"

	"strconv"

	"unicode/utf8"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/drvr/impl/common"
	"github.com/neilotoole/sq/libsq/drvr/scratch"
	"github.com/neilotoole/sq/libsq/shutdown"
	"github.com/neilotoole/sq/libsq/util"
)

const csvType = drvr.Type("csv")
const tsvType = drvr.Type("tsv")

type Driver struct {
	typ     drvr.Type
	mu      *sync.Mutex
	cleanup []func() error
}

func (d *Driver) Type() drvr.Type {
	return d.typ
}

func (d *Driver) ConnURI(source *drvr.Source) (string, error) {
	return "", util.Errorf("not implemented")
}

func (d *Driver) Open(src *drvr.Source) (*sql.DB, error) {

	lg.Debugf("attempting to ping file %q", src.Location)
	err := d.Ping(src)
	if err != nil {
		return nil, err
	}
	lg.Debugf("successfully pinged file %q", src.Location)
	// we now know that the xlsx file is valid

	// let's open the scratch db
	_, scratchdb, _, err := scratch.OpenNew()
	//shutdown.Add(cleanup) // TODO: re-enable cleanup
	if err != nil {
		return nil, err
	}

	lg.Debugf("opened scratch db: %s", src.String())

	err = d.csvToScratch(src, scratchdb)
	if err != nil {
		return nil, err
	}

	return scratchdb, nil
}

func (d *Driver) ValidateSource(src *drvr.Source) (*drvr.Source, error) {
	lg.Debugf("validating source: %q", src.Location)

	if src.Type != d.typ {
		return nil, util.Errorf("expected source type %q but got %q", d.typ, src.Type)
	}

	if src.Options != nil || len(src.Options) > 0 {
		lg.Debugf("opts: %v", src.Options.Encode())

		key := "header"
		v := src.Options.Get(key)

		if v != "" {
			_, err := strconv.ParseBool(v)
			if err != nil {
				return nil, util.Errorf(`unable to parse option %q: %v`, key, err)
			}
		}

	}

	return src, nil
}

func (d *Driver) Ping(src *drvr.Source) error {

	lg.Debugf("driver %q attempting to ping %q", d.Type(), src)

	file, _, cleanup, err := drvr.GetSourceFile(src.Location)
	shutdown.Add(cleanup)
	if err != nil {
		return err
	}

	lg.Debugf("file name: %q", file.Name())

	//if util.FileExists(file.Name())

	return nil
}

func (d *Driver) Metadata(src *drvr.Source) (*drvr.SourceMetadata, error) {

	lg.Debugf(src.String())

	return nil, util.Errorf("not implemented")
}

func init() {
	d := &Driver{typ: csvType, mu: &sync.Mutex{}}
	drvr.Register(d)
	d = &Driver{typ: tsvType, mu: &sync.Mutex{}}
	drvr.Register(d)
}

func (d *Driver) Release() error {

	d.mu.Lock()
	defer d.mu.Unlock()
	lg.Debugf("running driver cleanup tasks")

	errs := []string{}

	for _, cleaner := range d.cleanup {
		err := cleaner()
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		err := util.Errorf("cleanup error: %s", strings.Join(errs, "\n"))
		return err
	}

	lg.Debugf("driver cleanup tasks complete")
	return nil
}

// optDelimiter returns ok as true and the delimiter rune if a valid value is provided
// in src.Opts, returns ok as false if no valid value provided, and an error if the provided
// value is invalid.
func optDelimiter(src *drvr.Source) (r rune, ok bool, err error) {
	if len(src.Options) == 0 {
		return 0, false, nil
	}

	const key = "delim"
	_, ok = src.Options[key]
	if !ok {
		return 0, false, nil
	}
	val := src.Options.Get(key)
	if val == "" {
		return 0, false, nil
	}

	if len(val) == 1 {
		r, _ = utf8.DecodeRuneInString(val)
		return r, true, nil
	}

	r, ok = NamedDelimiters()[val]

	if !ok {
		err = util.Errorf("unknown delimiter constant %q", val)
		return 0, false, err
	}

	return r, true, nil
}

func (d *Driver) csvToScratch(src *drvr.Source, db *sql.DB) error {

	// Since CSVs only have one "table" of data, it's necessary to give this
	// table a name. Example: sq '@my_csv.data | .[0:10]'
	const tblName = "data"

	file, _, cleanup, err := drvr.GetSourceFile(src.Location)
	shutdown.Add(cleanup)
	if err != nil {
		return err
	}

	var insertStmt string
	// We add the CR filter reader to deal with files exported from Excel which
	// can have the DOS-style \r EOL markers.
	r := csv.NewReader(util.NewCRFilterReader(file))

	delim, ok, err := optDelimiter(src)
	if err != nil {
		return err
	}

	if ok {
		r.Comma = delim
	} else if d.typ == tsvType {
		r.Comma = '\t'
	}

	lg.Debugf("using delimiter '%v' for file: %s", string(r.Comma), src.Location)

	var readCount int64

	optHasHeader, _, err := common.OptionsHasHeader(src)
	if err != nil {
		return err
	}

	colNames, _, err := common.OptionsColNames(src)
	if err != nil {
		return err
	}

	if optHasHeader {
		record, err := r.Read()
		if err == io.EOF {
			return util.Errorf("data source %s has no data", src.Handle)
		}

		if err != nil {
			return util.WrapError(err)
		}

		// if option cols is provided, it takes precedence over the CSV header record
		if len(colNames) == 0 {
			colNames = record
		}
	}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return util.Errorf("read from data source %q: %v", src.Handle, err)
		}

		if readCount == 0 {

			if len(colNames) == 0 {
				// if no column names yet, we generate them
				colNames = make([]string, len(record))
				for i := range colNames {
					colNames[i] = drvr.GenerateAlphaColName(i)
				}
			}

			createStmt, err := d.tblCreateStmt(src, r, tblName, colNames)
			if err != nil {
				return err
			}

			lg.Debugf("creating table with SQL:\n%s", createStmt)
			_, err = db.Exec(createStmt)
			if err != nil {
				return util.WrapError(err)
			}

			escapedColNames := make([]string, len(colNames))
			for i, colName := range colNames {
				escapedColNames[i] = `"` + colName + `"`
			}

			placeholders := make([]string, len(colNames))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			insertTpl := `INSERT INTO "%s" ( %s ) VALUES ( %s )`
			insertStmt = fmt.Sprintf(insertTpl, tblName, strings.Join(escapedColNames, ", "), strings.Join(placeholders, ", "))
		}

		vals := make([]interface{}, len(record))
		for i := range record {
			vals[i] = record[i]
		}

		_, err = db.Exec(insertStmt, vals...)
		if err != nil {
			return util.WrapError(err)
		}

		readCount++
	}

	if readCount == 0 {
		return util.Errorf("data source %s is empty", src.Handle)
	}

	lg.Debugf("read %d records from %s", readCount, src.Handle)

	return nil

}

func (d *Driver) tblCreateStmt(src *drvr.Source, r *csv.Reader, tblName string, colNames []string) (string, error) {

	// create the table initially with all col types as TEXT
	colTypes := make([]string, len(colNames))
	colExprs := make([]string, len(colNames))
	for i := 0; i < len(colNames); i++ {
		colTypes[i] = AffinityText
		colExprs[i] = fmt.Sprintf(`"%s" %s`, colNames[i], colTypes[i])
	}

	tblTpl := `CREATE TABLE IF NOT EXISTS "%s" ( %s )`

	stmt := fmt.Sprintf(tblTpl, tblName, strings.Join(colExprs, ", "))
	lg.Debugf("creating scratch table using SQL: %s", stmt)
	return stmt, nil

}

func (d *Driver) getColNames(src *drvr.Source, r *csv.Reader, firstRecord []string) ([]string, error) {

	colNames := make([]string, len(firstRecord))

	for i := range colNames {
		colNames[i] = drvr.GenerateAlphaColName(i)
	}

	return colNames, nil
	// TODO: allow header column
}

// NamedDelimiters returns a map of named delimiter strings to their rune value.
// For example, "comma" maps to ',' and "pipe" maps to '|'.
func NamedDelimiters() map[string]rune {

	// TODO: save this in a var
	m := make(map[string]rune)
	m["comma"] = ','
	m["space"] = ' '
	m["pipe"] = '|'
	m["tab"] = '\t'
	m["colon"] = ':'
	m["semi"] = ';'
	m["period"] = '.'

	return m
}

const AffinityText = `TEXT`
const AffinityNumeric = `NUMERIC`
const AffinityInteger = `INTEGER`
const AffinityReal = `REAL`
const AffinityBlob = `BLOB`
