package mysql

import (
	"fmt"
	"strings"

	"github.com/neilotoole/go-lg/lg"
	"github.com/neilotoole/sq-driver/hackery/database/sql"
	_ "github.com/neilotoole/sq-driver/hackery/drivers/mysql"
	"github.com/neilotoole/sq/libsq/drvr"
	"github.com/neilotoole/sq/libsq/util"
)

const typ = drvr.Type("mysql")

type Driver struct {
}

func (d *Driver) Type() drvr.Type {
	return typ
}

func (d *Driver) ConnURI(src *drvr.Source) (string, error) {
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
		return nil, util.Errorf("expected source type %q but got %q", typ, src.Type)
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
	meta.Location = src.Location
	db, err := d.Open(src)
	if err != nil {
		return nil, util.WrapError(err)
	}

	q := `SELECT table_schema, SUM( data_length + index_length ) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()`
	row := db.QueryRow(q)
	err = row.Scan(&meta.Name, &meta.Size)
	if err != nil {
		return nil, util.WrapError(err)
	}

	meta.FullyQualifiedName = meta.Name

	q = "SELECT TABLE_SCHEMA AS `schema_name`,TABLE_NAME AS `table_name`, TABLE_COMMENT AS `table_comment`, (DATA_LENGTH + INDEX_LENGTH) AS `table_size` FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_NAME ASC"
	lg.Debugf("SQL: %s", q)
	rows, err := db.Query(q)
	if err != nil {
		return nil, util.WrapError(err)
	}
	defer db.Close()
	defer rows.Close()

	for rows.Next() {

		tbl := &drvr.Table{}

		err = rows.Scan(&meta.Name, &tbl.Name, &tbl.Comment, &tbl.Size)
		if err != nil {
			return nil, util.WrapError(err)
		}

		err = populateTblMetadata(db, meta.Name, tbl)
		if err != nil {
			return nil, err
		}

		meta.Tables = append(meta.Tables, *tbl)
	}

	return meta, nil
}

func populateTblMetadata(db *sql.DB, dbName string, tbl *drvr.Table) error {

	tpl := "SELECT column_name, data_type, column_type, ordinal_position, column_default, is_nullable, column_key, column_comment, extra, (SELECT COUNT(*) FROM `%s`) AS row_count FROM information_schema.columns cols WHERE cols.TABLE_SCHEMA = '%s' AND cols.TABLE_NAME = '%s' ORDER BY cols.ordinal_position ASC"
	q := fmt.Sprintf(tpl, tbl.Name, dbName, tbl.Name)

	lg.Debugf("SQL: %s", q)

	rows, err := db.Query(q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {

		col := &drvr.Column{}
		var isNullable, colKey, extra string
		//defVal := &sql.N
		defVal := &sql.NullString{}
		err = rows.Scan(&col.Name, &col.Datatype, &col.ColType, &col.Position, defVal, &isNullable, &colKey, &col.Comment, &extra, &tbl.RowCount)
		if err != nil {
			return util.WrapError(err)
		}

		if "YES" == strings.ToUpper(isNullable) {
			col.Nullable = true
		}

		if strings.Index(colKey, "PRI") != -1 {
			col.PrimaryKey = true
		}

		col.DefaultValue = defVal.String

		tbl.Columns = append(tbl.Columns, *col)
	}

	return nil
}

func init() {
	d := &Driver{}
	drvr.Register(d)
}
