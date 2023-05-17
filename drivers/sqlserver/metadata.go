package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/neilotoole/sq/libsq/core/options"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/driver"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/c2h5oh/datasize"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/source"
	"golang.org/x/sync/errgroup"
)

// kindFromDBTypeName determines the kind.Kind from the database
// type name. For example, "VARCHAR" -> kind.Text.
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
	var knd kind.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	switch dbTypeName {
	default:
		log.Warn("Unknown SQLServer database type '%s' for column '%s': using %s", dbTypeName, colName, kind.Unknown)
		knd = kind.Unknown
	case "INT", "BIGINT", "SMALLINT", "TINYINT":
		knd = kind.Int
	case "CHAR", "NCHAR", "VARCHAR", "JSON", "NVARCHAR", "NTEXT", "TEXT":
		knd = kind.Text
	case "BIT":
		knd = kind.Bool
	case "BINARY", "VARBINARY", "IMAGE":
		knd = kind.Bytes
	case "DECIMAL", "NUMERIC":
		knd = kind.Decimal
	case "MONEY", "SMALLMONEY":
		knd = kind.Decimal
	case "DATETIME", "DATETIME2", "SMALLDATETIME", "DATETIMEOFFSET":
		knd = kind.Datetime
	case "DATE":
		knd = kind.Date
	case "TIME":
		knd = kind.Time
	case "FLOAT", "REAL":
		knd = kind.Float
	case "XML":
		knd = kind.Text
	case "UNIQUEIDENTIFIER":
		knd = kind.Text
	case "ROWVERSION", "TIMESTAMP":
		knd = kind.Int
	}

	return knd
}

// setScanType does some manipulation of ct's scan type.
// Most importantly, if ct is nullable column, setwe  colTypeData.ScanType to a
// nullable type. This is because the driver doesn't
// report nullable scan types.
func setScanType(ct *sqlz.ColumnTypeData, knd kind.Kind) {
	if knd == kind.Decimal {
		// The driver wants us to use []byte instead of string for DECIMAL,
		// but we want to use string.
		if ct.Nullable {
			ct.ScanType = sqlz.RTypeNullString
		} else {
			ct.ScanType = sqlz.RTypeString
		}
		return
	}

	if !ct.Nullable {
		// If the col type is not nullable, there's nothing
		// to do here.
		return
	}

	switch ct.ScanType {
	default:
		ct.ScanType = sqlz.RTypeNullString

	case sqlz.RTypeInt64:
		ct.ScanType = sqlz.RTypeNullInt64

	case sqlz.RTypeBool:
		ct.ScanType = sqlz.RTypeNullBool

	case sqlz.RTypeFloat64:
		ct.ScanType = sqlz.RTypeNullFloat64

	case sqlz.RTypeString:
		ct.ScanType = sqlz.RTypeNullString

	case sqlz.RTypeTime:
		ct.ScanType = sqlz.RTypeNullTime

	case sqlz.RTypeBytes:
		ct.ScanType = sqlz.RTypeBytes // no change
	}
}

func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB) (*source.Metadata, error) {
	log := lg.FromContext(ctx)
	ctx = options.NewContext(ctx, src.Options)

	const query = `SELECT DB_NAME(), SCHEMA_NAME(), SERVERPROPERTY('ProductVersion'), @@VERSION,
(SELECT SUM(size) * 8192
FROM sys.master_files WITH(NOWAIT)
WHERE database_id = DB_ID()
GROUP BY database_id) AS total_size_bytes`

	md := &source.Metadata{Driver: Type, DBDriver: Type}
	md.Handle = src.Handle
	md.Location = src.Location

	var catalog, schema string
	err := db.QueryRowContext(ctx, query).
		Scan(&catalog, &schema, &md.DBVersion, &md.DBProduct, &md.Size)
	if err != nil {
		return nil, errz.Err(err)
	}

	md.Name = catalog
	md.FQName = catalog + "." + schema
	md.Schema = schema

	if md.DBSettings, err = getDBProperties(ctx, db); err != nil {
		return nil, err
	}

	tblNames, tblTypes, err := getAllTables(ctx, db)
	if err != nil {
		return nil, err
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(driver.OptTuningErrgroupLimit.Get(src.Options))
	tblMetas := make([]*source.TableMetadata, len(tblNames))
	for i := range tblNames {
		i := i
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			var tblMeta *source.TableMetadata
			tblMeta, err = getTableMetadata(gCtx, db, catalog, schema, tblNames[i], tblTypes[i])
			if err != nil {
				if hasErrCode(err, errCodeObjectNotExist) {
					// This can happen if the table is dropped while
					// we're collecting metadata. We log a warning and continue.
					log.Warn("Table metadata: table not found (continuing regardless)",
						lga.Table, tblNames[i],
						lga.Err, err,
					)

					return nil
				}

				return err
			}
			tblMetas[i] = tblMeta
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, errz.Err(err)
	}

	// If a table wasn't found (possibly dropped while querying), then
	// its entry could be nil. We copy the non-nil elements to the
	// final slice.
	md.Tables = make([]*source.TableMetadata, 0, len(tblMetas))
	for i := range tblMetas {
		if tblMetas[i] != nil {
			md.Tables = append(md.Tables, tblMetas[i])
		}
	}
	return md, nil
}

func getTableMetadata(ctx context.Context, db sqlz.DB, tblCatalog,
	tblSchema, tblName, tblType string,
) (*source.TableMetadata, error) {
	const tplTblUsage = `sp_spaceused '%s'`

	tblMeta := &source.TableMetadata{Name: tblName, DBTableType: tblType}
	tblMeta.FQName = tblCatalog + "." + tblSchema + "." + tblName

	switch tblMeta.DBTableType {
	case "BASE TABLE":
		tblMeta.TableType = sqlz.TableTypeTable
	case "VIEW":
		tblMeta.TableType = sqlz.TableTypeView
	default:
	}

	var rowCount, reserved, data, indexSize, unused sql.NullString
	row := db.QueryRowContext(ctx, fmt.Sprintf(tplTblUsage, tblName))
	err := row.Scan(&tblMeta.Name, &rowCount, &reserved, &data, &indexSize, &unused)
	if err != nil {
		return nil, errz.Err(err)
	}

	if rowCount.Valid {
		tblMeta.RowCount, err = strconv.ParseInt(strings.TrimSpace(rowCount.String), 10, 64)
		if err != nil {
			return nil, errz.Err(err)
		}
	} else {
		// We can't get the "row count" for a VIEW from sp_spaceused,
		// so we need to select it the old-fashioned way.
		err = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", tblName)).Scan(&tblMeta.RowCount)
		if err != nil {
			return nil, errz.Err(err)
		}
	}

	if reserved.Valid {
		var byteCount datasize.ByteSize
		err = byteCount.UnmarshalText([]byte(reserved.String))
		if err != nil {
			return nil, errz.Err(err)
		}
		size := int64(byteCount.Bytes())
		tblMeta.Size = &size
	}

	var dbCols []columnMeta
	dbCols, err = getColumnMeta(ctx, db, tblCatalog, tblSchema, tblName)
	if err != nil {
		return nil, err
	}

	var dbConstraints []constraintMeta
	dbConstraints, err = getConstraints(ctx, db, tblCatalog, tblSchema, tblName)
	if err != nil {
		return nil, errz.Err(err)
	}

	cols := make([]*source.ColMetadata, len(dbCols))
	for i := range dbCols {
		cols[i] = &source.ColMetadata{
			Name:         dbCols[i].ColumnName,
			Position:     dbCols[i].OrdinalPosition,
			BaseType:     dbCols[i].DataType,
			Kind:         kindFromDBTypeName(lg.FromContext(ctx), dbCols[i].ColumnName, dbCols[i].DataType),
			Nullable:     dbCols[i].Nullable.Bool,
			DefaultValue: dbCols[i].ColumnDefault.String,
		}

		// We want to output something like VARCHAR(255) for ColType

		// REVISIT: This is all a bit messy and inconsistent with other drivers
		var colLength *int64
		switch {
		case dbCols[i].CharMaxLength.Valid:
			colLength = &dbCols[i].CharMaxLength.Int64
		case dbCols[i].NumericPrecision.Valid:
			colLength = &dbCols[i].NumericPrecision.Int64
		case dbCols[i].DateTimePrecision.Valid:
			colLength = &dbCols[i].DateTimePrecision.Int64
		}

		if colLength != nil {
			cols[i].ColumnType = fmt.Sprintf("%s(%v)", dbCols[i].DataType, *colLength)
		} else {
			cols[i].ColumnType = dbCols[i].DataType
		}

		for _, dbConstraint := range dbConstraints {
			if dbCols[i].ColumnName == dbConstraint.ColumnName {
				if dbConstraint.ConstraintType == "PRIMARY KEY" {
					cols[i].PrimaryKey = true
					break
				}
			}
		}
	}

	tblMeta.Columns = cols
	return tblMeta, nil
}

// getAllTables returns all of the table names, and the table types
// (i.e. "BASE TABLE" or "VIEW").
func getAllTables(ctx context.Context, db sqlz.DB) (tblNames, tblTypes []string, err error) {
	log := lg.FromContext(ctx)

	const query = `SELECT TABLE_NAME, TABLE_TYPE FROM INFORMATION_SCHEMA.TABLES
WHERE TABLE_TYPE='BASE TABLE' OR TABLE_TYPE='VIEW'
ORDER BY TABLE_NAME ASC, TABLE_TYPE ASC`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	for rows.Next() {
		var tblName, tblType string
		err = rows.Scan(&tblName, &tblType)
		if err != nil {
			return nil, nil, errz.Err(err)
		}

		tblNames = append(tblNames, tblName)
		tblTypes = append(tblTypes, tblType)
	}

	if rows.Err() != nil {
		return nil, nil, errz.Err(rows.Err())
	}

	return tblNames, tblTypes, nil
}

func getColumnMeta(ctx context.Context, db sqlz.DB, tblCatalog, tblSchema, tblName string) ([]columnMeta, error) {
	log := lg.FromContext(ctx)

	// TODO: sq doesn't use all of these columns, no need to select them all.
	const query = `SELECT
		TABLE_CATALOG, TABLE_SCHEMA, TABLE_NAME,
		COLUMN_NAME, ORDINAL_POSITION, COLUMN_DEFAULT, IS_NULLABLE, DATA_TYPE,
		CHARACTER_MAXIMUM_LENGTH, CHARACTER_OCTET_LENGTH,
		NUMERIC_PRECISION, NUMERIC_PRECISION_RADIX, NUMERIC_SCALE,
		DATETIME_PRECISION,
		CHARACTER_SET_CATALOG, CHARACTER_SET_SCHEMA, CHARACTER_SET_NAME,
		COLLATION_CATALOG, COLLATION_SCHEMA, COLLATION_NAME,
		DOMAIN_CATALOG, DOMAIN_SCHEMA, DOMAIN_NAME
	FROM INFORMATION_SCHEMA.COLUMNS
	WHERE TABLE_CATALOG = @p1 AND TABLE_SCHEMA = @p2 AND TABLE_NAME = @p3`

	rows, err := db.QueryContext(ctx, query, tblCatalog, tblSchema, tblName)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	var cols []columnMeta

	for rows.Next() {
		c := columnMeta{}
		err = rows.Scan(&c.TableCatalog, &c.TableSchema, &c.TableName, &c.ColumnName, &c.OrdinalPosition,
			&c.ColumnDefault, &c.Nullable, &c.DataType, &c.CharMaxLength, &c.CharOctetLength, &c.NumericPrecision,
			&c.NumericPrecisionRadix, &c.NumericScale, &c.DateTimePrecision, &c.CharSetCatalog, &c.CharSetSchema,
			&c.CharSetName, &c.CollationCatalog, &c.CollationSchema, &c.CollationName, &c.DomainCatalog,
			&c.DomainSchema, &c.DomainName)
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

func getConstraints(ctx context.Context, db sqlz.DB, tblCatalog, tblSchema, tblName string) ([]constraintMeta, error) {
	log := lg.FromContext(ctx)

	const query = `SELECT kcu.TABLE_CATALOG, kcu.TABLE_SCHEMA, kcu.TABLE_NAME,  tc.CONSTRAINT_TYPE,
       kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME
		FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS AS tc
		  JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE AS kcu
			ON tc.TABLE_NAME = kcu.TABLE_NAME
			   AND tc.CONSTRAINT_CATALOG = kcu.CONSTRAINT_CATALOG
			   AND tc.CONSTRAINT_SCHEMA = kcu.CONSTRAINT_SCHEMA
			   AND tc.CONSTRAINT_NAME = kcu.CONSTRAINT_NAME
		WHERE tc.TABLE_CATALOG = @p1 AND tc.TABLE_SCHEMA = @p2 AND tc.TABLE_NAME = @p3
		ORDER BY kcu.TABLE_NAME, tc.CONSTRAINT_TYPE, kcu.CONSTRAINT_NAME`

	rows, err := db.QueryContext(ctx, query, tblCatalog, tblSchema, tblName)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	var constraints []constraintMeta

	for rows.Next() {
		c := constraintMeta{}
		err = rows.Scan(&c.TableCatalog, &c.TableSchema, &c.TableName, &c.ConstraintType, &c.ColumnName,
			&c.ConstraintName)
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

// constraintMeta models constraint metadata from information schema.
type constraintMeta struct {
	TableCatalog   string `db:"TABLE_CATALOG"`
	TableSchema    string `db:"TABLE_SCHEMA"`
	TableName      string `db:"TABLE_NAME"`
	ConstraintType string `db:"CONSTRAINT_TYPE"`
	ColumnName     string `db:"COLUMN_NAME"`
	ConstraintName string `db:"CONSTRAINT_NAME"`
}

// columnMeta models column metadata from information schema.
type columnMeta struct {
	TableCatalog          string         `db:"TABLE_CATALOG"`
	TableSchema           string         `db:"TABLE_SCHEMA"`
	TableName             string         `db:"TABLE_NAME"`
	ColumnName            string         `db:"COLUMN_NAME"`
	OrdinalPosition       int64          `db:"ORDINAL_POSITION"`
	ColumnDefault         sql.NullString `db:"COLUMN_DEFAULT"`
	Nullable              sqlz.NullBool  `db:"IS_NULLABLE"`
	DataType              string         `db:"DATA_TYPE"`
	CharMaxLength         sql.NullInt64  `db:"CHARACTER_MAXIMUM_LENGTH"`
	CharOctetLength       sql.NullString `db:"CHARACTER_OCTET_LENGTH"`
	NumericPrecision      sql.NullInt64  `db:"NUMERIC_PRECISION"`
	NumericPrecisionRadix sql.NullInt64  `db:"NUMERIC_PRECISION_RADIX"`
	NumericScale          sql.NullInt64  `db:"NUMERIC_SCALE"`
	DateTimePrecision     sql.NullInt64  `db:"DATETIME_PRECISION"`
	CharSetCatalog        sql.NullString `db:"CHARACTER_SET_CATALOG"`
	CharSetSchema         sql.NullString `db:"CHARACTER_SET_SCHEMA"`
	CharSetName           sql.NullString `db:"CHARACTER_SET_NAME"`
	CollationCatalog      sql.NullString `db:"COLLATION_CATALOG"`
	CollationSchema       sql.NullString `db:"COLLATION_SCHEMA"`
	CollationName         sql.NullString `db:"COLLATION_NAME"`
	DomainCatalog         sql.NullString `db:"DOMAIN_CATALOG"`
	DomainSchema          sql.NullString `db:"DOMAIN_SCHEMA"`
	DomainName            sql.NullString `db:"DOMAIN_NAME"`
}
