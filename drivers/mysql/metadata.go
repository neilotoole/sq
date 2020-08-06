package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
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
		log.Warnf("Unknown MySQL database type %s for column %s: instead using %s", dbTypeName, colName, sqlz.KindUnknown)
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

func getSourceMetadata(ctx context.Context, log lg.Log, src *source.Source, db *sql.DB) (*source.Metadata, error) {
	srcMeta := &source.Metadata{SourceType: Type, DBDriverType: Type}
	srcMeta.Handle = src.Handle
	srcMeta.Location = src.Location

	const versionQ = `SELECT @@GLOBAL.version, @@GLOBAL.version_comment, @@GLOBAL.version_compile_os, @@GLOBAL.version_compile_machine`
	var version, versionComment, versionOS, versionArch string
	err := db.QueryRowContext(ctx, versionQ).Scan(&version, &versionComment, &versionOS, &versionArch)
	if err != nil {
		return nil, errz.Err(err)
	}

	srcMeta.DBVersion = version
	srcMeta.DBProduct = fmt.Sprintf("%s %s / %s (%s)", versionComment, version, versionOS, versionArch)

	varRows, err := db.QueryContext(ctx, "SHOW VARIABLES")
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(varRows)
	for varRows.Next() {
		var dbVar source.DBVar
		err = varRows.Scan(&dbVar.Name, &dbVar.Value)
		if err != nil {
			return nil, errz.Err(err)
		}
		srcMeta.DBVars = append(srcMeta.DBVars, dbVar)
	}
	err = varRows.Err()
	if err != nil {
		return nil, errz.Err(err)
	}

	// get the schema name and total table size
	const schemaSummaryQ = `SELECT table_schema, SUM( data_length + index_length ) AS table_size
		FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()`
	err = db.QueryRowContext(ctx, schemaSummaryQ).Scan(&srcMeta.Name, &srcMeta.Size)
	if err != nil {
		return nil, errz.Err(err)
	}
	srcMeta.FQName = srcMeta.Name

	const tblSummaryQ = `SELECT TABLE_NAME, TABLE_TYPE, TABLE_COMMENT,  (DATA_LENGTH + INDEX_LENGTH) AS TABLE_SIZE
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = DATABASE() ORDER BY TABLE_SCHEMA, TABLE_NAME ASC`
	tblSchemaRows, err := db.QueryContext(ctx, tblSummaryQ)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(tblSchemaRows)

	var tblSize sql.NullInt64
	for tblSchemaRows.Next() {
		tblMeta := &source.TableMetadata{}

		err = tblSchemaRows.Scan(&tblMeta.Name, &tblMeta.TableType, &tblMeta.Comment, &tblSize)
		if err != nil {
			return nil, errz.Err(err)
		}

		if tblSize.Valid {
			// for a view (as opposed to table), tblSize can be NULL
			tblMeta.Size = tblSize.Int64
		} else {
			tblMeta.Size = -1
		}

		err = populateTblMetadata(log, db, srcMeta.Name, tblMeta)
		if err != nil {
			if hasErrCode(err, errNumTableNotExist) {
				// If the table is dropped while we're collecting metadata,
				// for example, we log a warning and continue.
				log.Warnf("table metadata collection: table %q appears not to exist (continuing regardless): %v", tblMeta.Name, err)
				continue
			}

			return nil, err
		}

		srcMeta.Tables = append(srcMeta.Tables, tblMeta)
	}

	err = tblSchemaRows.Err()
	if err != nil {
		return nil, errz.Err(err)
	}

	return srcMeta, nil
}

func populateTblMetadata(log lg.Log, db *sql.DB, dbName string, tbl *source.TableMetadata) error {
	const tpl = "SELECT column_name, data_type, column_type, ordinal_position, column_default, is_nullable, column_key," +
		" column_comment, extra, (SELECT COUNT(*) FROM `%s`) AS row_count FROM information_schema.columns cols" +
		" WHERE cols.TABLE_SCHEMA = '%s' AND cols.TABLE_NAME = '%s'" +
		" ORDER BY cols.ordinal_position ASC"

	query := fmt.Sprintf(tpl, tbl.Name, dbName, tbl.Name)

	rows, err := db.Query(query)
	if err != nil {
		return errz.Err(err)
	}
	defer log.WarnIfCloseError(rows)

	for rows.Next() {
		col := &source.ColMetadata{}
		var isNullable, colKey, extra string

		defVal := &sql.NullString{}
		err = rows.Scan(&col.Name, &col.BaseType, &col.ColumnType, &col.Position, defVal, &isNullable, &colKey, &col.Comment, &extra, &tbl.RowCount)
		if err != nil {
			return errz.Err(err)
		}

		if strings.EqualFold("YES", isNullable) {
			col.Nullable = true
		}

		if strings.Contains(colKey, "PRI") {
			col.PrimaryKey = true
		}

		col.DefaultValue = defVal.String
		col.Kind = kindFromDBTypeName(log, col.Name, col.BaseType)

		tbl.Columns = append(tbl.Columns, col)
	}

	return errz.Err(rows.Err())
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
