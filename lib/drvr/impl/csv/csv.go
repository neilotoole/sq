package csv

import (
	"strings"

	"sync"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/drvr/drvrutil"
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
	_, scratchdb, err := scratch.OpenNew()
	if err != nil {
		return nil, err
	}

	lg.Debugf("opened handle to scratch db")

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

	file, _, cleanup, err := drvrutil.GetSourceFile(src.Location)
	if err != nil {
		return err
	}

	shutdown.Add(cleanup)

	lg.Debugf("file name: %q", file.Name())

	//if util.FileExists(file.Name())

	return nil
}

func (d *Driver) Metadata(src *drvr.Source) (*drvr.SourceMetadata, error) {

	lg.Debugf(src)

	return nil, util.Errorf("not implemented")

	//
	//meta := &drvr.SourceMetadata{}
	//meta.Handle = src.Handle
	//
	//file, err := d.GetSourceFile(src)
	//if err != nil {
	//	return nil, err
	//}
	//defer file.Close()
	//
	//fi, err := os.Stat(file.Name())
	//if err != nil {
	//	return nil, util.WrapError(err)
	//}
	//lg.Debugf("size: %d", fi.Size())
	//meta.Size = fi.Size()
	//
	//meta.Name, err = d.GetSourceFileName(src)
	//if err != nil {
	//	return nil, err
	//}
	//
	//meta.FullyQualifiedName = meta.Name
	//meta.Location = src.Location
	//
	////lg.Debugf("source file name: %s", d.getSourceFileName(src))
	//
	//xlFile, err := xlsx.OpenFile(file.Name())
	//if err != nil {
	//	return nil, util.Errorf("unable to open XLSX file %q: %v", file.Name(), err)
	//}
	//
	////sheets := xlFile.Sheets
	//
	//for _, sheet := range xlFile.Sheets {
	//	tbl := drvr.Table{}
	//
	//	tbl.Name = sheet.Name
	//	tbl.Size = -1
	//	tbl.RowCount = int64(len(sheet.Rows))
	//
	//	colTypes := getColTypes(sheet)
	//
	//	for i, colType := range colTypes {
	//
	//		col := drvr.Column{}
	//		col.Datatype = cellTypeToString(colType)
	//		col.ColType = col.Datatype
	//		col.Position = int64(i)
	//		col.Name = GenerateExcelColName(i)
	//		tbl.Columns = append(tbl.Columns, col)
	//	}
	//
	//	meta.Tables = append(meta.Tables, tbl)
	//
	//}
	//
	//return meta, nil

	//return nil, util.Errorf("not implemented")
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

	return util.Errorf("not implemented")

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
}
