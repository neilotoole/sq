package sqlite3

import (
	_ "github.com/neilotoole/sq-driver/hackery/database/drivers/go-sqlite3"

	"fmt"

	"os"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	"github.com/neilotoole/sq/lib/drvr"
	"github.com/neilotoole/sq/lib/util"
)

const typ = drvr.Type("sqlite3")

type Driver struct {
}

func (d *Driver) Type() drvr.Type {
	return typ
}

func (d *Driver) ConnURI(source *drvr.Source) (string, error) {
	return "", util.Errorf("not implemented")
}

func (d *Driver) Open(src *drvr.Source) (*sql.DB, error) {
	return sql.Open(string(src.Type), src.ConnURI())
}

func (d *Driver) Release() error {
	return nil
}

func (d *Driver) ValidateSource(src *drvr.Source) (*drvr.Source, error) {
	if src.Type != typ {
		return nil, util.Errorf("expected driver type %q but got %q", typ, src.Type)
	}
	return src, nil
}

func (d *Driver) Ping(src *drvr.Source) error {
	db, err := d.Open(src)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}

func (d *Driver) Metadata(src *drvr.Source) (*drvr.SourceMetadata, error) {

	meta := &drvr.SourceMetadata{}
	meta.Handle = src.Handle

	q := "SELECT tbl_name FROM sqlite_master WHERE type = 'table'"

	db, err := d.Open(src)
	if err != nil {
		return nil, err
	}
	lg.Debugf("SQL: %q", q)
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	defer rows.Close()

	fi, err := os.Stat(src.ConnURI())
	if err != nil {
		return nil, util.WrapError(err)
	}
	lg.Debugf("size: %d", fi.Size())
	meta.Size = fi.Size()
	meta.Name = fi.Name()
	meta.FullyQualifiedName = fi.Name()
	meta.Location = src.Location

	for rows.Next() {

		tbl := &drvr.Table{}

		err = rows.Scan(&tbl.Name)
		if err != nil {
			return nil, util.WrapError(err)
		}

		//lg.Debugf("tbl.Name: %v", tbl.Name)

		err = populateTblMetadata(db, meta.Name, tbl)
		if err != nil {
			return nil, err
		}

		meta.Tables = append(meta.Tables, *tbl)
	}
	return meta, nil
}

func populateTblMetadata(db *sql.DB, dbName string, tbl *drvr.Table) error {

	// No easy way of getting size of table
	tbl.Size = -1
	tpl := "SELECT COUNT(*) FROM '%s'"
	q := fmt.Sprintf(tpl, tbl.Name)
	lg.Debugf("SQL: %s", q)

	row := db.QueryRow(q)
	err := row.Scan(&tbl.RowCount)
	if err != nil {
		return util.WrapError(err)
	}

	tpl = "PRAGMA TABLE_INFO('%s')"
	q = fmt.Sprintf(tpl, tbl.Name)
	lg.Debugf("SQL: %s", q)

	rows, err := db.Query(q)
	if err != nil {
		return util.WrapError(err)
	}
	defer rows.Close()

	for rows.Next() {

		col := &drvr.Column{}

		var notnull int64
		defVal := &sql.NullString{}
		err = rows.Scan(&col.Position, &col.Name, &col.Datatype, &notnull, defVal, &col.PrimaryKey)
		if err != nil {
			return util.WrapError(err)
		}

		col.ColType = col.Datatype
		col.Nullable = notnull == 0
		col.DefaultValue = defVal.String
		tbl.Columns = append(tbl.Columns, *col)
	}

	return nil
}

func init() {
	d := &Driver{}
	drvr.Register(d)
}
