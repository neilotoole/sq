package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/c2h5oh/datasize"
	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlz"
)

// database implements driver.Database.
type database struct {
	log  lg.Log
	drvr *Driver
	db   *sql.DB
	src  *source.Source
}

var _ driver.Database = (*database)(nil)

// DB implements driver.Database.
func (d *database) DB() *sql.DB {
	return d.db
}

// SQLDriver implements driver.Database.
func (d *database) SQLDriver() driver.SQLDriver {
	return d.drvr
}

// Source implements driver.Database.
func (d *database) Source() *source.Source {
	return d.src
}

// TableMetadata implements driver.Database.
func (d *database) TableMetadata(ctx context.Context, tblName string) (*source.TableMetadata, error) {
	srcMeta, err := d.SourceMetadata(ctx)
	if err != nil {
		return nil, err
	}
	return source.TableFromSourceMetadata(srcMeta, tblName)
}

// SourceMetadata implements driver.Database.
func (d *database) SourceMetadata(ctx context.Context) (*source.Metadata, error) {
	const queryNameSize = `SELECT DB_NAME(), total_size_bytes = SUM(size) * 8192
FROM sys.master_files WITH(NOWAIT)
WHERE database_id = DB_ID() -- for current db
GROUP BY database_id;`

	srcMeta := &source.Metadata{SourceType: Type, DBDriverType: Type}
	srcMeta.Handle = d.src.Handle
	srcMeta.Location = d.src.Location
	db := d.db

	row := db.QueryRow(queryNameSize)
	err := row.Scan(&srcMeta.Name, &srcMeta.Size)
	if err != nil {
		return nil, errz.Err(err)
	}

	row = db.QueryRow("SELECT SERVERPROPERTY('ProductVersion'), @@VERSION")
	err = row.Scan(&srcMeta.DBVersion, &srcMeta.DBProduct)
	if err != nil {
		return nil, errz.Err(err)
	}

	query := "SELECT TABLE_CATALOG, TABLE_SCHEMA, TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE='BASE TABLE'"

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer d.log.WarnIfCloseError(rows)

	for rows.Next() {
		tblMeta := &source.TableMetadata{}
		var tblCatalog, tblSchema, tblName string

		err = rows.Scan(&tblCatalog, &tblSchema, &tblName)
		if err != nil {
			return nil, errz.Err(err)
		}

		srcMeta.Name = tblCatalog
		tblMeta.Name = tblName

		err = d.populateTblMetadata(ctx, db, tblCatalog, tblSchema, tblName, tblMeta)
		if err != nil {
			if hasErrCode(err, errCodeObjectNotExist) {
				// If the table is dropped while we're collecting metadata,
				// for example, we log a warning and continue.
				d.log.Warnf("table metadata collection: table %q appears not to exist (continuing regardless): %v", tblMeta.Name, err)
				continue
			}
			return nil, err
		}

		srcMeta.Tables = append(srcMeta.Tables, tblMeta)
	}

	if rows.Err() != nil {
		return nil, errz.Err(rows.Err())
	}

	return srcMeta, nil
}

// Close implements driver.Database.
func (d *database) Close() error {
	d.log.Debugf("Close database: %s", d.src)

	return errz.Err(d.db.Close())
}

func selectInfoColumns(ctx context.Context, log lg.Log, db sqlz.DB, tblCatalog, tblSchema, tblName string) ([]columnMeta, error) {
	// TODO: sq doesn't use all of these columns, no need to select them all.

	const tplSchemaCol = `SELECT
		TABLE_CATALOG, TABLE_SCHEMA, TABLE_NAME,
		COLUMN_NAME, ORDINAL_POSITION, COLUMN_DEFAULT, IS_NULLABLE, DATA_TYPE,
		CHARACTER_MAXIMUM_LENGTH, CHARACTER_OCTET_LENGTH,
		NUMERIC_PRECISION, NUMERIC_PRECISION_RADIX, NUMERIC_SCALE,
		DATETIME_PRECISION,
		CHARACTER_SET_CATALOG, CHARACTER_SET_SCHEMA, CHARACTER_SET_NAME,
		COLLATION_CATALOG, COLLATION_SCHEMA, COLLATION_NAME,
		DOMAIN_CATALOG, DOMAIN_SCHEMA, DOMAIN_NAME
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_CATALOG = '%s' AND TABLE_SCHEMA = '%s' AND TABLE_NAME = '%s'` // TODO: use sql args

	query := fmt.Sprintf(tplSchemaCol, tblCatalog, tblSchema, tblName)
	var err error
	var rows *sql.Rows

	rows, err = db.QueryContext(ctx, query)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer func() { log.WarnIfCloseError(rows) }()

	var cols []columnMeta

	for rows.Next() {
		c := columnMeta{}
		err = rows.Scan(&c.TableCatalog, &c.TableSchema, &c.TableName, &c.ColumnName, &c.OrdinalPosition,
			&c.ColumnDefault, &c.Nullable, &c.DataType, &c.CharMaxLength, &c.CharOctetLength, &c.NumericPrecision,
			&c.NumericPrecisionRadix, &c.NumericScale, &c.DateTimePrecision, &c.CharSetCatalog, &c.CharSetSchema,
			&c.CharSetName, &c.CollationCatalog, &c.CollationSchema, &c.CollationName, &c.DomainCatalog, &c.DomainSchema, &c.DomainName)
		if err != nil {
			return nil, errz.Err(err)
		}

		cols = append(cols, c)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return cols, nil
}

func getConstraints(ctx context.Context, log lg.Log, db sqlz.DB, tblCatalog, tblSchema, tblName string) ([]constraintMeta, error) {
	const tpl = `SELECT kcu.TABLE_CATALOG, kcu.TABLE_SCHEMA, kcu.TABLE_NAME,  tc.CONSTRAINT_TYPE, kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS AS tc
		  JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE AS kcu
			ON tc.TABLE_NAME = kcu.TABLE_NAME
			   AND tc.CONSTRAINT_CATALOG = kcu.CONSTRAINT_CATALOG
			   AND tc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA
			   AND tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		WHERE tc.TABLE_CATALOG = '%s' AND tc.TABLE_SCHEMA = '%s' AND tc.TABLE_NAME = '%s'
		ORDER BY kcu.TABLE_NAME, tc.CONSTRAINT_TYPE, kcu.CONSTRAINT_NAME`

	query := fmt.Sprintf(tpl, tblCatalog, tblSchema, tblName)
	var err error
	var rows *sql.Rows

	rows, err = db.QueryContext(ctx, query)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer func() { log.WarnIfCloseError(rows) }()

	var constraints []constraintMeta

	for rows.Next() {
		c := constraintMeta{}
		err = rows.Scan(&c.TableCatalog, &c.TableSchema, &c.TableName, &c.ConstraintType, &c.ColumnName, &c.ConstraintName)
		if err != nil {
			return nil, errz.Err(err)
		}

		constraints = append(constraints, c)
	}

	if err = rows.Err(); err != nil {
		return nil, errz.Err(err)
	}

	return constraints, nil
}

func (d *database) populateTblMetadata(ctx context.Context, db *sql.DB, tblCatalog, tblSchema, tblName string, tbl *source.TableMetadata) error {
	const tplTblUsage = `sp_spaceused '%s'`

	var rowCount, reserved, data, indexSize, unused string
	row := db.QueryRow(fmt.Sprintf(tplTblUsage, tblName))
	err := row.Scan(&tbl.Name, &rowCount, &reserved, &data, &indexSize, &unused)
	if err != nil {
		return errz.Err(err)
	}

	tbl.RowCount, err = strconv.ParseInt(strings.TrimSpace(rowCount), 10, 64)
	if err != nil {
		return errz.Err(err)
	}

	var byteCount datasize.ByteSize
	err = byteCount.UnmarshalText([]byte(reserved))
	if err != nil {
		return errz.Err(err)
	}
	tbl.Size = int64(byteCount.Bytes())

	var dbCols []columnMeta
	dbCols, err = selectInfoColumns(ctx, d.log, db, tblCatalog, tblSchema, tblName)
	if err != nil {
		return err
	}

	var dbConstraints []constraintMeta
	dbConstraints, err = getConstraints(ctx, d.log, db, tblCatalog, tblSchema, tblName)
	if err != nil {
		return errz.Err(err)
	}

	cols := make([]*source.ColMetadata, len(dbCols))
	for i := range dbCols {
		cols[i] = &source.ColMetadata{
			Name:         dbCols[i].ColumnName,
			Position:     dbCols[i].OrdinalPosition,
			BaseType:     dbCols[i].DataType,
			Kind:         kindFromDBTypeName(d.log, dbCols[i].ColumnName, dbCols[i].DataType),
			Nullable:     dbCols[i].Nullable.Bool,
			DefaultValue: dbCols[i].ColumnDefault.String,
		}

		// We want to output something like VARCHAR(255) for ColType
		var colLength *int64
		if dbCols[i].CharMaxLength.Valid {
			colLength = &dbCols[i].CharMaxLength.Int64
		} else if dbCols[i].NumericPrecision.Valid {
			colLength = &dbCols[i].NumericPrecision.Int64
		} else if dbCols[i].DateTimePrecision.Valid {
			colLength = &dbCols[i].DateTimePrecision.Int64
		}

		if colLength != nil {
			cols[i].ColumnType = fmt.Sprintf("%s(%v)", dbCols[i].DataType, *colLength)
		} else {
			cols[i].ColumnType = dbCols[i].DataType
		}

		for _, iConstraint := range dbConstraints {
			if dbCols[i].ColumnName == iConstraint.ColumnName {
				if iConstraint.ConstraintType == "PRIMARY KEY" {
					cols[i].PrimaryKey = true
					break
				}
			}
		}
	}

	tbl.Columns = cols
	return nil
}
