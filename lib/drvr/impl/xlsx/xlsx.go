package xslx

import (
	"strings"

	"fmt"

	"bytes"

	"unicode/utf8"

	"net/http"

	"io"
	"sync"

	"io/ioutil"

	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/drvr/scratch"
	"github.com/neilotoole/sq/lib/util"
	"github.com/tealeg/xlsx"
)

const typ = drvr.Type("xlsx")

type Driver struct {
	mu      *sync.Mutex
	cleanup []func() error
}

func (d *Driver) Type() drvr.Type {
	return typ
}

func (d *Driver) ConnURI(source *drvr.Source) (string, error) {
	return "", util.Errorf("not implemented")
}

func (d *Driver) Open(src *drvr.Source) (*sql.DB, error) {

	lg.Debugf("attempting to ping XLSX file %q", src.Location)
	err := d.Ping(src)
	if err != nil {
		return nil, err
	}
	lg.Debugf("successfully pinged XLSX file %q", src.Location)
	// we now know that the xlsx file is valid

	// let's open the scratch db
	_, scratchdb, err := scratch.OpenNew()
	if err != nil {
		return nil, err
	}

	lg.Debugf("opened handled to scratch db")

	err = d.xlsxToScratch(src, scratchdb)
	if err != nil {
		return nil, err
	}

	return scratchdb, nil
}

func (d *Driver) ValidateSource(src *drvr.Source) (*drvr.Source, error) {
	if src.Type != typ {
		return nil, util.Errorf("expected source type %q but got %q", typ, src.Type)
	}

	lg.Debugf("validating source: %q", src.Location)

	return src, nil
}

func (d *Driver) Ping(src *drvr.Source) error {

	lg.Debugf("driver %q attempting to ping %q", d.Type(), src)
	file, err := d.getSourceFile(src)
	if err != nil {
		return err
	}

	lg.Debugf("file exists: %q", file.Name())
	xlFile, err := xlsx.OpenFile(file.Name())
	if err != nil {
		return util.Errorf("unable to open XLSX file %q: %v", file.Name(), err)
	}

	lg.Debugf("successfully opened XLSX file %q with sheet count: %d", file.Name(), len(xlFile.Sheets))

	return nil
}

func (d *Driver) Metadata(src *drvr.Source) (*drvr.SourceMetadata, error) {

	meta := &drvr.SourceMetadata{}
	meta.Handle = src.Handle

	file, err := d.getSourceFile(src)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fi, err := os.Stat(file.Name())
	if err != nil {
		return nil, util.WrapError(err)
	}
	lg.Debugf("size: %d", fi.Size())
	meta.Size = fi.Size()

	meta.Name, err = d.getSourceFileName(src)
	if err != nil {
		return nil, err
	}

	meta.FullyQualifiedName = meta.Name
	meta.Location = src.Location

	//lg.Debugf("source file name: %s", d.getSourceFileName(src))

	xlFile, err := xlsx.OpenFile(file.Name())
	if err != nil {
		return nil, util.Errorf("unable to open XLSX file %q: %v", file.Name(), err)
	}

	//sheets := xlFile.Sheets

	for _, sheet := range xlFile.Sheets {
		tbl := drvr.Table{}

		tbl.Name = sheet.Name
		tbl.Size = -1
		tbl.RowCount = int64(len(sheet.Rows))

		colTypes := getColTypes(sheet)

		for i, colType := range colTypes {

			col := drvr.Column{}
			col.Datatype = cellTypeToString(colType)
			col.ColType = col.Datatype
			col.Position = int64(i)
			col.Name = genExcelColName(i)
			tbl.Columns = append(tbl.Columns, col)
		}

		meta.Tables = append(meta.Tables, tbl)

	}

	return meta, nil

	//return nil, util.Errorf("not implemented")
}

func cellTypeToString(typ xlsx.CellType) string {

	switch typ {
	case xlsx.CellTypeString:
		return "string"
	case xlsx.CellTypeFormula:
		return "formula"
	case xlsx.CellTypeNumeric:
		return "numeric"
	case xlsx.CellTypeBool:
		return "bool"
	case xlsx.CellTypeInline:
		return "inline"
	case xlsx.CellTypeError:
		return "error"
	case xlsx.CellTypeDate:
		return "date"
	}

	return "general"
}

func getColTypes(sheet *xlsx.Sheet) []xlsx.CellType {

	types := make([]*xlsx.CellType, len(sheet.Cols))

	for _, row := range sheet.Rows {

		for i, cell := range row.Cells {

			if types[i] == nil {
				typ := cell.Type()
				types[i] = &typ
				continue
			}

			// else, it already has a type
			if *types[i] == cell.Type() {
				// type matches, just continue
				continue
			}

			// it already has a type, and it's different from this cell's type
			typ := xlsx.CellTypeGeneral
			types[i] = &typ
		}
	}

	// convert back to value types
	ret := make([]xlsx.CellType, len(types))
	for i, typ := range types {
		ret[i] = *typ
	}

	return ret
}

func init() {
	d := &Driver{mu: &sync.Mutex{}}
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

// getSourceFileName returns the final component of the file/URL path.
func (d *Driver) getSourceFileName(src *drvr.Source) (string, error) {

	sep := os.PathSeparator
	if strings.HasPrefix(src.Location, "http") {
		sep = '/'
	}

	parts := strings.Split(src.Location, string(sep))
	if len(parts) == 0 || len(parts[len(parts)-1]) == 0 {
		return "", util.Errorf("illegal src [%s] location: %s", src.Handle, src.Location)
	}

	return parts[len(parts)-1], nil
}

// getSourceFile returns a file handle for XLSX file. The return file is open,
// the caller is responsible for closing it.
func (d *Driver) getSourceFile(src *drvr.Source) (*os.File, error) {

	// xlsx:///Users/neilotoole/sq/test/testdata.xlsx
	//`https://s3.amazonaws.com/sq.neilotoole.io/testdata/1.0/xslx/test.xlsx`
	//`/Users/neilotoole/nd/go/src/github.com/neilotoole/sq/test/xlsx/test.xlsx`

	lg.Debugf("attempting to determine XLSX filepath from source location %q", src.Location)

	if strings.HasPrefix(src.Location, "http://") || strings.HasPrefix(src.Location, "https://") {
		lg.Debugf("attempting to fetch XLSX file from %q", src.Location)

		resp, err := http.Get(src.Location)
		if err != nil {
			return nil, util.Errorf("unable to fetch XLSX file for datasource %q with location %q due to error: %v", src.Handle, src.Location, err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, util.Errorf("unable to fetch XLSX file for datasource %q with location %q due to HTTP status: %s/%d", src.Handle, src.Location, resp.Status, resp.StatusCode)
		}

		lg.Debugf("success fetching remote XLSX file from %q", src.Location)

		tmpFile, err := ioutil.TempFile("", "sq_xlsx_") // really should give this a suffix
		if err != nil {
			return nil, util.Errorf("unable to create tmp file: %v", err)
		}

		_, err = io.Copy(tmpFile, resp.Body)
		if err != nil {
			return nil, util.Errorf("error reading XLSX file from %q to %q", src.Location, tmpFile.Name())
		}
		defer resp.Body.Close()

		// TODO: revisit locking here
		d.mu.Lock()
		defer d.mu.Unlock()
		d.cleanup = append(d.cleanup, func() error {
			lg.Debugf("deleting tmp file %q", tmpFile.Name())
			return os.Remove(tmpFile.Name())
		})

		return tmpFile, nil

	}

	// If it's not remote, it should be a local path
	file, err := os.Open(src.Location)
	if err != nil {
		return nil, util.Errorf("error opening XLSX file %q: %v", src.Location, err)
	}

	return file, nil
}

func (d *Driver) xlsxToScratch(src *drvr.Source, db *sql.DB) error {
	file, err := d.getSourceFile(src)
	if err != nil {
		return err
	}
	defer file.Close()

	xlFile, err := xlsx.OpenFile(file.Name())
	if err != nil {
		return util.Errorf("unable to open XLSX file %q: %v", file.Name(), err)
	}

	//sheets := xlFile.Sheets

	for _, sheet := range xlFile.Sheets {

		lg.Debugf("attempting to create table for sheet %q", sheet.Name)
		colNames, err := createTblForSheet(db, sheet)
		if err != nil {
			return err
		}
		lg.Debugf("successfully created table for sheet %q", sheet.Name)

		escapedColNames := make([]string, len(colNames))
		for i, colName := range colNames {
			escapedColNames[i] = `"` + colName + `"`
		}

		//placeholders := strings.Repeat("?", len(colNames))
		placeholders := make([]string, len(colNames))
		for i, _ := range placeholders {
			placeholders[i] = "?"
		}

		insertTpl := `INSERT INTO "%s" ( %s ) VALUES ( %s )`
		insertStmt := fmt.Sprintf(insertTpl, sheet.Name, strings.Join(escapedColNames, ", "), strings.Join(placeholders, ", "))

		lg.Debugf("using INSERT stmt: %s", insertStmt)
		for _, row := range sheet.Rows {

			//result, err = database.Exec("Insert into Persons (id, LastName, FirstName, Address, City) values (?, ?, ?, ?, ?)", nil, "soni", "swati", "110 Eastern drive", "Mountain view, CA")
			vals := make([]interface{}, len(row.Cells))
			for i, cell := range row.Cells {
				typ := cell.Type()
				switch typ {
				case xlsx.CellTypeBool:
					vals[i] = cell.Bool()
				case xlsx.CellTypeNumeric:
					intVal, err := cell.Int64()
					if err == nil {
						vals[i] = intVal
						continue
					}
					floatVal, err := cell.Float()
					if err == nil {
						vals[i] = floatVal
						continue
					}
					// it's not an int, it's not a float, just give up and make it a string
					vals[i] = cell.Value

				case xlsx.CellTypeDate:
					//val, _ := cell.
					// TODO: parse into a time value here
					vals[i] = cell.Value
				default:
					vals[i] = cell.Value
				}

			}

			vls := make([]string, len(vals))
			for i, val := range vals {
				vls[i] = fmt.Sprintf("%v", val)
			}

			//lg.Debugf("INSERT INTO %q VALUES (%s)", sheet.Name, strings.Join(vls, ", "))

			_, err := db.Exec(insertStmt, vals...)
			if err != nil {
				return util.WrapError(err)
			}
		}
	}

	return nil
}

func getDBColTypeFromCell(cells []*xlsx.Cell) []string {

	vals := make([]string, len(cells))
	for i, cell := range cells {
		typ := cell.Type()
		switch typ {
		case xlsx.CellTypeBool:
			vals[i] = AffinityInteger
		case xlsx.CellTypeNumeric:
			_, err := cell.Int64()
			if err == nil {
				vals[i] = AffinityInteger
				continue
			}
			_, err = cell.Float()
			if err == nil {
				vals[i] = AffinityReal
				continue
			}
			// it's not an int, it's not a float
			vals[i] = AffinityNumeric

		case xlsx.CellTypeDate:
			// TODO: support time values here?
			vals[i] = AffinityText
		default:
			vals[i] = AffinityText
		}

	}

	return vals

}

const AffinityText = `TEXT`
const AffinityNumeric = `NUMERIC`
const AffinityInteger = `INTEGER`
const AffinityReal = `REAL`
const AffinityBlob = `BLOB`

// createTblForSheet creates a table for the given sheet, and returns an arry
// of the table's column names, or an error.
func createTblForSheet(db *sql.DB, sheet *xlsx.Sheet) ([]string, error) {

	lg.Debugf("creating table for sheet %q", sheet.Name)
	numCols := len(sheet.Cols)

	colNames := make([]string, numCols)
	colTypes := make([]string, numCols)
	colExprs := make([]string, numCols)

	if len(sheet.Rows) == 0 {
		for i := 0; i < numCols; i++ {
			colTypes[i] = "TEXT"
		}
	} else {
		colTypes = getDBColTypeFromCell(sheet.Rows[0].Cells)
	}

	//firstRow := sheet.Rows[0]
	//firstRow.Cells
	//
	//for i, cell := range firstRow.Cells {
	//	typ := cell.Type()
	//}

	for i := 0; i < numCols; i++ {
		colNames[i] = genExcelColName(i)
		//colTypes[i] = "TEXT"
		colExprs[i] = fmt.Sprintf(`"%s" %s`, colNames[i], colTypes[i])
	}

	lg.Debugf("using col names [%q]", strings.Join(colNames, ", "))

	// need to get the col types

	//tblTpl :=	"CREATE TABLE IF NOT EXISTS Persons ( id integer PRIMARY KEY, LastName varchar(255) NOT NULL, FirstName varchar(255), Address varchar(255), City varchar(255), CONSTRAINT uc_PersonID UNIQUE (id,LastName))",)
	tblTpl := `CREATE TABLE IF NOT EXISTS "%s" ( %s )`

	stmt := fmt.Sprintf(tblTpl, sheet.Name, strings.Join(colExprs, ", "))
	lg.Debugf("creating table for sheet %q using stmt: %s", sheet.Name, stmt)

	_, err := db.Exec(stmt)
	if err != nil {
		return nil, util.WrapError(err)
	}

	lg.Debugf("created table %q", sheet.Name)

	return colNames, nil
}

// genExcelColNames will return an Excel style list of col names, starting with A, B, C...
// and continuing to AA, AB, AC, etc...

func genExcelColName(n int) string {
	return genCol(n, 'A', 26)
}

func genCol(n int, start rune, lenAlpha int) string {

	buf := &bytes.Buffer{}

	for ; n >= 0; n = int(n/lenAlpha) - 1 {

		buf.WriteRune(rune(n%lenAlpha) + start)
	}

	return reverse(buf.String())
}

func reverse(s string) string {
	size := len(s)
	buf := make([]byte, size)
	for start := 0; start < size; {
		r, n := utf8.DecodeRuneInString(s[start:])
		start += n
		utf8.EncodeRune(buf[size-start:], r)
	}
	return string(buf)
}
