package csv

import (
	"strings"

	"sync"

	"encoding/csv"

	"fmt"
	"io"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/drvr/scratch"
	"github.com/neilotoole/sq/lib/shutdown"
	"github.com/neilotoole/sq/lib/util"
)

const typ = drvr.Type("csv")

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
	if src.Type != typ {
		return nil, util.Errorf("expected source type %q but got %q", typ, src.Type)
	}

	lg.Debugf("validating source: %q", src.Location)

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

func (d *Driver) csvToScratch(src *drvr.Source, db *sql.DB) error {

	const tblName = "data"

	file, _, cleanup, err := drvr.GetSourceFile(src.Location)
	shutdown.Add(cleanup)
	if err != nil {
		return err
	}

	//var escapedColNames []string
	//var placeholders []string
	var insertStmt string
	r := csv.NewReader(file)
	var readCount int64

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return util.Errorf("unable to read data source %q: %v", src.Location, err)
		}

		if readCount == 0 {
			colNames, err := d.getColNames(src, r, record)
			if err != nil {
				return err
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

const AffinityText = `TEXT`
const AffinityNumeric = `NUMERIC`
const AffinityInteger = `INTEGER`
const AffinityReal = `REAL`
const AffinityBlob = `BLOB`

//file, err := d.GetSourceFile(src)
//if err != nil {
//	return err
//}
//defer file.Close()
//
//r := csv.NewReader(file)
//
//for {
//	record, err := r.Read()
//	if err == io.EOF {
//		break
//	}
//	if err != nil {
//		return util.WrapError(err)
//	}
//
//	fmt.Println(record)
//}
//
//xlFile, err := xlsx.OpenFile(file.Name())
//if err != nil {
//	return util.Errorf("unable to open file %q: %v", file.Name(), err)
//}
//
////sheets := xlFile.Sheets
//
//for _, sheet := range xlFile.Sheets {
//
//	lg.Debugf("attempting to create table for sheet %q", sheet.Name)
//	colNames, err := createTblForSheet(db, sheet)
//	if err != nil {
//		return err
//	}
//	lg.Debugf("successfully created table for sheet %q", sheet.Name)
//
//	escapedColNames := make([]string, len(colNames))
//	for i, colName := range colNames {
//		escapedColNames[i] = `"` + colName + `"`
//	}
//
//	//placeholders := strings.Repeat("?", len(colNames))
//	placeholders := make([]string, len(colNames))
//	for i, _ := range placeholders {
//		placeholders[i] = "?"
//	}
//
//	insertTpl := `INSERT INTO "%s" ( %s ) VALUES ( %s )`
//	insertStmt := fmt.Sprintf(insertTpl, sheet.Name, strings.Join(escapedColNames, ", "), strings.Join(placeholders, ", "))
//
//	lg.Debugf("using INSERT stmt: %s", insertStmt)
//	for _, row := range sheet.Rows {
//
//		//result, err = database.Exec("Insert into Persons (id, LastName, FirstName, Address, City) values (?, ?, ?, ?, ?)", nil, "soni", "swati", "110 Eastern drive", "Mountain view, CA")
//		vals := make([]interface{}, len(row.Cells))
//		for i, cell := range row.Cells {
//			typ := cell.Type()
//			switch typ {
//			case xlsx.CellTypeBool:
//				vals[i] = cell.Bool()
//			case xlsx.CellTypeNumeric:
//				intVal, err := cell.Int64()
//				if err == nil {
//					vals[i] = intVal
//					continue
//				}
//				floatVal, err := cell.Float()
//				if err == nil {
//					vals[i] = floatVal
//					continue
//				}
//				// it's not an int, it's not a float, just give up and make it a string
//				vals[i] = cell.Value
//
//			case xlsx.CellTypeDate:
//				//val, _ := cell.
//				// TODO: parse into a time value here
//				vals[i] = cell.Value
//			default:
//				vals[i] = cell.Value
//			}
//
//		}
//
//		vls := make([]string, len(vals))
//		for i, val := range vals {
//			vls[i] = fmt.Sprintf("%v", val)
//		}
//
//		//lg.Debugf("INSERT INTO %q VALUES (%s)", sheet.Name, strings.Join(vls, ", "))
//
//		_, err := db.Exec(insertStmt, vals...)
//		if err != nil {
//			return util.WrapError(err)
//		}
//	}
//}
//
//return nil
