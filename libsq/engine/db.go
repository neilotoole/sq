package engine

import (
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/libsq/drvr"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq/libsq/drvr/sqlh"
	"github.com/neilotoole/sq/libsq/util"
)

// Database encapsulates a SQL database.
type Database struct {
	src *drvr.Source
	drv drvr.Driver
}

// NewDatabase returns a new Database instance for the provided data source.
func NewDatabase(src *drvr.Source) (*Database, error) {
	drv, err := drvr.For(src)
	if err != nil {
		return nil, err
	}
	return &Database{src: src, drv: drv}, nil
}

// Query executes the SQL query against the database, and writes the resutls to writer.
func (d *Database) Query(sql string, writer RecordWriter) error {
	lg.Debugf("attempting to open SQL connection for datasource %q with query: %s", d.src, sql)
	db, err := d.drv.Open(d.src)
	if err != nil {
		return err
	}
	defer db.Close()

	err = d.exec(db, sql, writer)
	if err != nil {
		return err
	}
	return nil
}

// Ping pings the database, verifying that the connection is healthy.
func (d *Database) Ping() error {
	lg.Debugf("attempting to open SQL connection for datasource %q for ping", d.src)
	return d.drv.Ping(d.src)
}

func (d *Database) exec(db *sql.DB, query string, writer RecordWriter) error {

	rows, err := db.Query(query)
	if err != nil {
		return util.WrapError(err)
	}
	defer rows.Close()

	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return util.WrapError(err)
	}

	records := []*sqlh.Record{}

	for rows.Next() {
		rr, err := sqlh.NewRecord(colTypes)
		if err != nil {
			return err
		}

		err = rows.Scan(rr.Values...)
		if err != nil {
			return util.WrapError(err)
		}

		records = append(records, rr)
	}
	if rows.Err() != nil {
		return util.WrapError(rows.Err())
	}

	err = writer.Records(records)
	if err != nil {
		return err
	}
	return writer.Close()
}
