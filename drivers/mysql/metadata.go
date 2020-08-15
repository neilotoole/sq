package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/neilotoole/errgroup"
	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/errz"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/sqlz"
)

// kindFromDBTypeName determines the sqlz.Kind from the database
// type name. For example, "VARCHAR(64)" -> sqlz.KindText.
func kindFromDBTypeName(log lg.Log, colName, dbTypeName string) sqlz.Kind {
	var kind sqlz.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	// Given variations such as VARCHAR(255), we first trim the parens
	// parts. Thus VARCHAR(255) becomes VARCHAR.
	i := strings.IndexRune(dbTypeName, '(')
	if i > 0 {
		dbTypeName = dbTypeName[0:i]
	}

	switch dbTypeName {
	default:
		log.Warnf("Unknown MySQL database type %q for column %q: using %q", dbTypeName, colName, sqlz.KindUnknown)
		kind = sqlz.KindUnknown
	case "":
		kind = sqlz.KindUnknown
	case "INTEGER", "INT", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT", "YEAR", "BIT":
		kind = sqlz.KindInt
	case "DECIMAL", "NUMERIC":
		kind = sqlz.KindDecimal
	case "CHAR", "VARCHAR", "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT":
		kind = sqlz.KindText
	case "ENUM", "SET":
		kind = sqlz.KindText
	case "JSON":
		kind = sqlz.KindText
	case "VARBINARY", "BINARY", "BLOB", "MEDIUMBLOB", "LONGBLOB", "TINYBLOB":
		kind = sqlz.KindBytes
	case "DATETIME", "TIMESTAMP":
		kind = sqlz.KindDatetime
	case "DATE":
		kind = sqlz.KindDate
	case "TIME":
		kind = sqlz.KindTime
	case "FLOAT", "DOUBLE", "DOUBLE PRECISION", "REAL":
		kind = sqlz.KindFloat
	case "BOOL", "BOOLEAN":
		// In practice these are not returned by the mysql driver.
		kind = sqlz.KindBool
	}

	return kind
}

func recordMetaFromColumnTypes(log lg.Log, colTypes []*sql.ColumnType) sqlz.RecordMeta {
	recMeta := make(sqlz.RecordMeta, len(colTypes))

	for i, colType := range colTypes {
		kind := kindFromDBTypeName(log, colType.Name(), colType.DatabaseTypeName())
		colTypeData := sqlz.NewColumnTypeData(colType, kind)
		recMeta[i] = sqlz.NewFieldMeta(colTypeData)
	}

	return recMeta
}

// getNewRecordFunc returns a NewRecordFunc that, after interacting
// with the standard driver.NewRecordFromScanRow, munges any skipped fields.
// In particular mysql.NullTime is unboxed to *time.Time, and TIME fields
// are munged from RawBytes to string.
func getNewRecordFunc(rowMeta sqlz.RecordMeta) driver.NewRecordFunc {
	return func(row []interface{}) (sqlz.Record, error) {
		rec, skipped := driver.NewRecordFromScanRow(rowMeta, row, nil)
		// We iterate over each element of val, checking for certain
		// conditions. A more efficient approach might be to (in
		// the outside func) iterate over the column metadata, and
		// build a list of val elements to visit.
		for _, i := range skipped {
			if nullTime, ok := rec[i].(*mysql.NullTime); ok {
				if nullTime.Valid {
					// Make a copy of the value
					t := nullTime.Time
					rec[i] = &t
					continue
				}

				// Else
				rec[i] = nil
				continue
			}

			if rowMeta[i].DatabaseTypeName() == "TIME" && rec[i] != nil {
				// MySQL may return TIME as RawBytes... convert to a string.
				// https://github.com/go-sql-driver/mysql#timetime-support
				if rb, ok := rec[i].(*sql.RawBytes); ok {
					if len(*rb) == 0 {
						// shouldn't happen
						zero := "00:00"
						rec[i] = &zero
						continue
					}

					// Else
					text := string(*rb)
					rec[i] = &text
				}

				continue
			}

			// else, we don't know what to do with this col
			return nil, errz.Errorf("column %d %s: unknown type db(%T) with kind(%s), val(%v)", i, rowMeta[i].Name(), rec[i], rowMeta[i].Kind(), rec[i])
		}
		return rec, nil
	}
}

func getSourceMetadata(ctx context.Context, log lg.Log, src *source.Source, db sqlz.DB) (*source.Metadata, error) {
	md := &source.Metadata{SourceType: Type, DBDriverType: Type, Handle: src.Handle, Location: src.Location}

	const summaryQuery = `SELECT @@GLOBAL.version, @@GLOBAL.version_comment, @@GLOBAL.version_compile_os,
       @@GLOBAL.version_compile_machine, DATABASE(), CURRENT_USER(),
       (SELECT SUM( data_length + index_length )
        FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()) AS size`

	var version, versionComment, versionOS, versionArch, schema string
	err := db.QueryRowContext(ctx, summaryQuery).Scan(&version, &versionComment, &versionOS, &versionArch, &schema, &md.User, &md.Size)
	if err != nil {
		return nil, errz.Err(err)
	}

	md.Name = schema
	md.FQName = schema
	md.DBVersion = version
	md.DBProduct = fmt.Sprintf("%s %s / %s (%s)", versionComment, version, versionOS, versionArch)

	md.DBVars, err = getDBVarsMeta(ctx, log, db)
	if err != nil {
		return nil, err
	}

	// Note that this does not populate the RowCount of Columns fields of the
	// table metadata.
	tblMetas, err := getSchemaTableMetas(ctx, log, db, schema)
	if err != nil {
		return nil, err
	}

	// Populate the RowCount and Columns fields of each table metadata.
	// Note that this function may set elements of tblMetas to nil
	// if the table is not found (can happen if a table is dropped
	// during metadata collection).
	err = setTableMetaDetails(ctx, log, db, tblMetas)
	if err != nil {
		return nil, err
	}

	// Filter any nil tables
	md.Tables = make([]*source.TableMetadata, 0, len(tblMetas))
	for i := range tblMetas {
		if tblMetas[i] != nil {
			md.Tables = append(md.Tables, tblMetas[i])
		}
	}

	return md, nil
}

func getTableMetadata(ctx context.Context, log lg.Log, db sqlz.DB, tblName string) (*source.TableMetadata, error) {
	query := `SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE, TABLE_COMMENT, (DATA_LENGTH + INDEX_LENGTH) AS table_size,
(SELECT COUNT(*) FROM ` + "`" + tblName + "`" + `) AS row_count
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`

	var schema string
	var tblSize sql.NullInt64
	tblMeta := &source.TableMetadata{}

	err := db.QueryRowContext(ctx, query, tblName).
		Scan(&schema, &tblMeta.Name, &tblMeta.DBTableType, &tblMeta.Comment, &tblSize, &tblMeta.RowCount)
	if err != nil {
		return nil, errz.Err(err)
	}

	tblMeta.TableType = canonicalTableType(tblMeta.DBTableType)
	tblMeta.FQName = schema + "." + tblMeta.Name
	if tblSize.Valid {
		// For a view (as opposed to table), tblSize is typically nil
		tblMeta.Size = &tblSize.Int64
	}

	tblMeta.Columns, err = getColumnMetadata(ctx, log, db, tblMeta.Name)
	if err != nil {
		return nil, err
	}

	return tblMeta, nil
}

// getSchemaTableMetas returns basic metadata for each table in schema. Note
// that the returned items are not fully populated: column metadata
// must be separately populated.
func getSchemaTableMetas(ctx context.Context, log lg.Log, db sqlz.DB, schema string) ([]*source.TableMetadata, error) {
	const query = `SELECT TABLE_NAME, TABLE_TYPE, TABLE_COMMENT,  (DATA_LENGTH + INDEX_LENGTH) AS table_size
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_SCHEMA, TABLE_NAME ASC`

	rows, err := db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(rows)

	var tblMetas []*source.TableMetadata
	for rows.Next() {
		tblMeta := &source.TableMetadata{}

		var tblSize sql.NullInt64
		err = rows.Scan(&tblMeta.Name, &tblMeta.DBTableType, &tblMeta.Comment, &tblSize)
		if err != nil {
			return nil, errz.Err(err)
		}

		tblMeta.TableType = canonicalTableType(tblMeta.DBTableType)
		tblMeta.FQName = schema + "." + tblMeta.Name
		if tblSize.Valid {
			// For a view (as opposed to table), tblSize is typically nil
			tblMeta.Size = &tblSize.Int64
		}

		tblMetas = append(tblMetas, tblMeta)
	}

	err = rows.Err()
	if err != nil {
		return nil, errz.Err(err)
	}

	return tblMetas, nil
}

// setTableMetaDetails sets the RowCount and Columns field on each
// of tblMetas. It can happen that a table in tblMetas is dropped
// during the metadata collection process: if so, that element of
// tblMetas is set to nil.
func setTableMetaDetails(ctx context.Context, log lg.Log, db sqlz.DB, tblMetas []*source.TableMetadata) error {
	g, gctx := errgroup.WithContextN(ctx, driver.Tuning.ErrgroupNumG, driver.Tuning.ErrgroupQSize)
	for i := range tblMetas {
		i := i

		g.Go(func() error {
			err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `"+tblMetas[i].Name+"`").Scan(&tblMetas[i].RowCount)
			if err != nil {
				if hasErrCode(err, errNumTableNotExist) {
					// Can happen if the table is dropped while we're collecting metadata,
					log.Warnf("table metadata: table %q appears not to exist (continuing regardless): %v", tblMetas[i].Name, err)

					// We'll need to delete this nil entry below
					tblMetas[i] = nil
					return nil
				}
			}

			cols, err := getColumnMetadata(gctx, log, db, tblMetas[i].Name)
			if err != nil {
				if hasErrCode(err, errNumTableNotExist) {
					log.Warnf("table metadata: table %q appears not to exist (continuing regardless): %v", tblMetas[i].Name, err)
					tblMetas[i] = nil
					return nil
				}

				return err
			}

			tblMetas[i].Columns = cols
			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return errz.Err(err)
	}
	return nil
}

// getColumnMetadata returns column metadata for tblName.
func getColumnMetadata(ctx context.Context, log lg.Log, db sqlz.DB, tblName string) ([]*source.ColMetadata, error) {
	const query = `SELECT column_name, data_type, column_type, ordinal_position, column_default, is_nullable, column_key, column_comment, extra
FROM information_schema.columns cols
WHERE cols.TABLE_SCHEMA = DATABASE() AND cols.TABLE_NAME = ?
ORDER BY cols.ordinal_position ASC`

	rows, err := db.QueryContext(ctx, query, tblName)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(rows)

	var cols []*source.ColMetadata

	for rows.Next() {
		col := &source.ColMetadata{}
		var isNullable, colKey, extra string

		defVal := &sql.NullString{}
		err = rows.Scan(&col.Name, &col.BaseType, &col.ColumnType, &col.Position, defVal, &isNullable, &colKey, &col.Comment, &extra)
		if err != nil {
			return nil, errz.Err(err)
		}

		if strings.EqualFold("YES", isNullable) {
			col.Nullable = true
		}

		if strings.Contains(colKey, "PRI") {
			col.PrimaryKey = true
		}

		col.DefaultValue = defVal.String
		col.Kind = kindFromDBTypeName(log, col.Name, col.BaseType)

		cols = append(cols, col)
	}

	return cols, errz.Err(rows.Err())
}

// getDBVarsMeta returns the database variables.
func getDBVarsMeta(ctx context.Context, log lg.Log, db sqlz.DB) ([]source.DBVar, error) {
	var dbVars []source.DBVar

	rows, err := db.QueryContext(ctx, "SHOW VARIABLES")
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(rows)

	for rows.Next() {
		var dbVar source.DBVar
		err = rows.Scan(&dbVar.Name, &dbVar.Value)
		if err != nil {
			return nil, errz.Err(err)
		}
		dbVars = append(dbVars, dbVar)
	}
	err = rows.Err()
	if err != nil {
		return nil, errz.Err(err)
	}

	return dbVars, nil
}

// newInsertMungeFunc is lifted from driver.DefaultInsertMungeFunc.
func newInsertMungeFunc(destTbl string, destMeta sqlz.RecordMeta) driver.InsertMungeFunc {
	return func(rec sqlz.Record) error {
		if len(rec) != len(destMeta) {
			return errz.Errorf("insert record has %d vals but dest table %s has %d cols (%s)",
				len(rec), destTbl, len(destMeta), strings.Join(destMeta.Names(), ","))
		}

		for i := range rec {
			nullable, _ := destMeta[i].Nullable()
			if rec[i] == nil && !nullable {
				mungeSetZeroValue(i, rec, destMeta)
				continue
			}

			if destMeta[i].Kind() == sqlz.KindText {
				// text doesn't need our help
				continue
			}

			// The dest col kind is something other than text, let's inspect
			// the actual value and check its type.
			switch val := rec[i].(type) {
			default:
				continue
			case string:
				if val == "" {
					if nullable {
						rec[i] = nil
					} else {
						mungeSetZeroValue(i, rec, destMeta)
					}
				}
				// else we let the DB figure it out

			case *string:
				if *val == "" {
					if nullable {
						rec[i] = nil
					} else {
						mungeSetZeroValue(i, rec, destMeta)
					}
				}

				// string is non-empty
				if destMeta[i].Kind() == sqlz.KindDatetime {
					// special handling for datetime
					mungeSetDatetimeFromString(*val, i, rec)
				}

				// else we let the DB figure it out
			}
		}
		return nil
	}
}

// datetimeLayouts are layouts attempted with time.Parse to
// try to give mysql a time.Time instead of string.
var datetimeLayouts = []string{time.RFC3339Nano, time.RFC3339}

// mungeSetDatetimeFromString attempts to parse s into time.Time and
// sets rec[i] to that value. If unable to parse, rec is unchanged,
// and it's up to mysql to deal with the text.
func mungeSetDatetimeFromString(s string, i int, rec []interface{}) {
	var t time.Time
	var err error

	for _, layout := range datetimeLayouts {
		t, err = time.Parse(layout, s)
		if err == nil {
			rec[i] = t
			return
		}
	}
}

// mungeSetZeroValue is invoked when rec[i] is nil, but
// destMeta[i] is not nullable.
func mungeSetZeroValue(i int, rec []interface{}, destMeta sqlz.RecordMeta) {
	// REVISIT: do we need to do special handling for kind.Datetime
	//  and kind.Time (e.g. "00:00" for time)?
	z := reflect.Zero(destMeta[i].ScanType()).Interface()
	rec[i] = z
}

// canonicalTableType returns the canonical name for "BASE TABLE"
// and "VIEW"
func canonicalTableType(dbType string) string {
	switch dbType {
	default:
		return ""
	case "BASE TABLE":
		return sqlz.TableTypeTable
	case "VIEW":
		return sqlz.TableTypeView
	}
}
