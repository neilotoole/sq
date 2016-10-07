package ql

import (
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	_ "github.com/neilotoole/sq/lib/driver/impl"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/lib/common"
	"github.com/neilotoole/sq/lib/driver"
	"github.com/neilotoole/sq/lib/out"
	"github.com/neilotoole/sq/lib/util"
)

type Database struct {
	src  *driver.Source
	drvr driver.Driver
}

func NewDatabase(src *driver.Source) (*Database, error) {

	lg.Debugf("attempting to get driver for datasource %q", src)
	drvr, err := driver.For(src)
	if err != nil {
		return nil, err
	}
	return &Database{src: src, drvr: drvr}, nil
}

func (d *Database) ExecuteAndWrite(query string, writer out.ResultWriter) error {

	//db, err := sql.Open(string(d.src.Type), d.src.ConnURI())
	lg.Debugf("attempting to open SQL connection for datasource %q with query: %q", d.src, query)
	db, err := d.drvr.Open(d.src)
	if err != nil {
		return err
	}
	defer db.Close()

	err = d.exec(db, query, writer)
	if err != nil {
		return err
	}
	return nil
}

// Execute runs the query against the database. The caller is responsible for closing
// the returned DB and Rows.
func (d *Database) Execute(query string) (*sql.DB, *sql.Rows, error) {

	//db, err := sql.Open(string(d.src.Type), d.src.ConnURI())
	lg.Debugf("attempting to open SQL connection for datasource %q with query: %q", d.src, query)
	db, err := d.drvr.Open(d.src)
	if err != nil {
		return nil, nil, err
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, nil, err
	}

	return db, rows, nil
}

func (d *Database) Ping() error {

	lg.Debugf("attempting to open SQL connection for datasource %q for ping", d.src)

	return d.drvr.Ping(d.src)

}

func (d *Database) exec(db *sql.DB, query string, writer out.ResultWriter) error {

	rows, err := db.Query(query)
	if err != nil {
		return util.WrapError(err)
	}
	defer rows.Close()
	fields, err := rows.ColumnTypes()
	if err != nil {
		return util.WrapError(err)
	}

	lg.Debugf("with fields: %s", fields)

	//for _, f := range fields {
	//	fmt.Println(f)
	//}

	scannedRows := []*common.ResultRow{}

	for rows.Next() {

		// TODO: look into rawbytes
		//sql.RawBytes{}
		rr := common.NewResultRow(fields)

		err = rows.Scan(rr.Values...)
		if err != nil {
			return util.WrapError(err)
		}

		scannedRows = append(scannedRows, rr)
	}
	if rows.Err() != nil {
		return util.WrapError(rows.Err())
	}

	err = writer.ResultRows(scannedRows)
	return err

}
