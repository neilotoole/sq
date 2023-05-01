package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/lg/lga"

	"github.com/neilotoole/sq/libsq/core/lg/lgm"

	"github.com/neilotoole/sq/libsq/core/lg"

	"golang.org/x/exp/slog"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// kindFromDBTypeName determines the Kind from the database
// type name. For example, "VARCHAR(64)" -> kind.Text.
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
	var knd kind.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	// Given variations such as VARCHAR(255), we first trim the parens
	// parts. Thus VARCHAR(255) becomes VARCHAR.
	i := strings.IndexRune(dbTypeName, '(')
	if i > 0 {
		dbTypeName = dbTypeName[0:i]
	}

	switch dbTypeName {
	default:
		log.Warn(
			"Unknown MySQL column type: using alt type",
			lga.DBType, dbTypeName,
			lga.Col, colName,
			lga.Alt, kind.Unknown,
		)

		knd = kind.Unknown
	case "":
		knd = kind.Unknown
	case "INTEGER", "INT", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT", "YEAR", "BIT",
		"UNSIGNED INTEGER", "UNSIGNED INT", "UNSIGNED TINYINT",
		"UNSIGNED SMALLINT", "UNSIGNED MEDIUMINT", "UNSIGNED BIGINT":
		knd = kind.Int
	case "DECIMAL", "NUMERIC":
		knd = kind.Decimal
	case "CHAR", "VARCHAR", "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT":
		knd = kind.Text
	case "ENUM", "SET":
		knd = kind.Text
	case "JSON":
		knd = kind.Text
	case "VARBINARY", "BINARY", "BLOB", "MEDIUMBLOB", "LONGBLOB", "TINYBLOB":
		knd = kind.Bytes
	case "DATETIME", "TIMESTAMP":
		knd = kind.Datetime
	case "DATE":
		knd = kind.Date
	case "TIME": //nolint:goconst
		knd = kind.Time
	case "FLOAT", "DOUBLE", "DOUBLE PRECISION", "REAL":
		knd = kind.Float
	case "BOOL", "BOOLEAN":
		// In practice these are not returned by the mysql driver.
		knd = kind.Bool
	}

	return knd
}

func recordMetaFromColumnTypes(log *slog.Logger, colTypes []*sql.ColumnType) sqlz.RecordMeta {
	recMeta := make(sqlz.RecordMeta, len(colTypes))

	for i, colType := range colTypes {
		knd := kindFromDBTypeName(log, colType.Name(), colType.DatabaseTypeName())
		colTypeData := sqlz.NewColumnTypeData(colType, knd)
		recMeta[i] = sqlz.NewFieldMeta(colTypeData)
	}

	return recMeta
}

// getNewRecordFunc returns a NewRecordFunc that, after interacting
// with the standard driver.NewRecordFromScanRow, munges any skipped fields.
// In particular sql.NullTime is unboxed to *time.Time, and TIME fields
// are munged from RawBytes to string.
func getNewRecordFunc(rowMeta sqlz.RecordMeta) driver.NewRecordFunc {
	return func(row []any) (sqlz.Record, error) {
		rec, skipped := driver.NewRecordFromScanRow(rowMeta, row, nil)
		// We iterate over each element of val, checking for certain
		// conditions. A more efficient approach might be to (in
		// the outside func) iterate over the column metadata, and
		// build a list of val elements to visit.
		for _, i := range skipped {
			if nullTime, ok := rec[i].(*sql.NullTime); ok {
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
			return nil, errz.Errorf("column %d %s: unknown type db(%T) with kind(%s), val(%v)", i, rowMeta[i].Name(),
				rec[i], rowMeta[i].Kind(), rec[i])
		}
		return rec, nil
	}
}

// getTableMetadata gets the metadata for a single table. It is the
// implementation of driver.Database.TableMetadata.
func getTableMetadata(ctx context.Context, db sqlz.DB, tblName string) (*source.TableMetadata, error) {
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

	tblMeta.Columns, err = getColumnMetadata(ctx, db, tblMeta.Name)
	if err != nil {
		return nil, err
	}

	return tblMeta, nil
}

// getColumnMetadata returns column metadata for tblName.
func getColumnMetadata(ctx context.Context, db sqlz.DB, tblName string) ([]*source.ColMetadata, error) {
	log := lg.From(ctx)

	const query = `SELECT column_name, data_type, column_type, ordinal_position, column_default,
       is_nullable, column_key, column_comment, extra
FROM information_schema.columns cols
WHERE cols.TABLE_SCHEMA = DATABASE() AND cols.TABLE_NAME = ?
ORDER BY cols.ordinal_position ASC`

	rows, err := db.QueryContext(ctx, query, tblName)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	var cols []*source.ColMetadata

	for rows.Next() {
		col := &source.ColMetadata{}
		var isNullable, colKey, extra string

		defVal := &sql.NullString{}
		err = rows.Scan(&col.Name, &col.BaseType, &col.ColumnType, &col.Position, defVal, &isNullable, &colKey,
			&col.Comment, &extra)
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

// getSourceMetadata is the implementation of driver.Database.SourceMetadata.
//
// Multiple queries are required to build the SourceMetadata, and this
// impl makes use of errgroup to make concurrent queries. In the initial
// relatively sequential implementation of this function, the main perf
// roadblock was getting the row count for each table/view. For accuracy,
// it is necessary to perform "SELECT COUNT(*) FROM tbl" for each table/view.
// For other databases (such as sqlite) it was performant to UNION ALL
// these SELECTs into one (or a few) queries, e.g.:
//
//	SELECT COUNT(*) FROM actor
//	UNION ALL
//	SELECT COUNT(*) FROM address
//	UNION ALL
//	[...]
//
// However, this seemed to perform poorly (at least for MySQL 5.6 which
// was the main focus of testing). We do seem to be getting fairly
// reasonable results by spinning off a goroutine (via errgroup) for
// each SELECT COUNT(*) query. That said, the testing/benchmarking was
// far from exhaustive, and this entire thing has a bit of a code smell.
func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB) (*source.Metadata, error) {
	md := &source.Metadata{
		SourceType:   Type,
		DBDriverType: Type,
		Handle:       src.Handle,
		Location:     src.Location,
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(driver.Tuning.ErrgroupLimit)

	g.Go(func() error {
		return doRetry(gCtx, src.Options, func() error {
			return setSourceSummaryMeta(gCtx, db, md)
		})
	})

	g.Go(func() error {
		return doRetry(gCtx, src.Options, func() error {
			var err error
			md.DBVars, err = getDBVarsMeta(gCtx, db)
			return err
		})
	})

	g.Go(func() error {
		return doRetry(gCtx, src.Options, func() error {
			var err error
			md.Tables, err = getAllTblMetas(gCtx, db)
			return err
		})
	})

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	return md, nil
}

func setSourceSummaryMeta(ctx context.Context, db sqlz.DB, md *source.Metadata) error {
	const summaryQuery = `SELECT @@GLOBAL.version, @@GLOBAL.version_comment, @@GLOBAL.version_compile_os,
       @@GLOBAL.version_compile_machine, DATABASE(), CURRENT_USER(),
       (SELECT SUM( data_length + index_length )
        FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()) AS size`

	var version, versionComment, versionOS, versionArch, schema string
	err := db.QueryRowContext(ctx, summaryQuery).Scan(&version, &versionComment, &versionOS, &versionArch, &schema,
		&md.User, &md.Size)
	if err != nil {
		return errz.Err(err)
	}

	md.Name = schema
	md.Schema = schema
	md.FQName = schema
	md.DBVersion = version
	md.DBProduct = fmt.Sprintf("%s %s / %s (%s)", versionComment, version, versionOS, versionArch)
	return nil
}

// getDBVarsMeta returns the database variables.
func getDBVarsMeta(ctx context.Context, db sqlz.DB) ([]source.DBVar, error) {
	log := lg.From(ctx)
	var dbVars []source.DBVar

	rows, err := db.QueryContext(ctx, "SHOW VARIABLES")
	if err != nil {
		return nil, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

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

// getAllTblMetas returns TableMetadata for each table/view in db.
//
// NOTE: getAllTblMetas has the potential to become deadlocked. It spawns
// a number of goroutines, which can bump up against the max conn limit.
// This function should be revisited to see if there's a better way
// to implement it.
func getAllTblMetas(ctx context.Context, db sqlz.DB) ([]*source.TableMetadata, error) {
	log := lg.From(ctx)

	const query = `SELECT t.TABLE_SCHEMA, t.TABLE_NAME, t.TABLE_TYPE, t.TABLE_COMMENT,
       (DATA_LENGTH + INDEX_LENGTH) AS table_size,
       c.COLUMN_NAME, c.ORDINAL_POSITION, c.COLUMN_KEY, c.DATA_TYPE, c.COLUMN_TYPE,
       c.IS_NULLABLE, c.COLUMN_DEFAULT, c.COLUMN_COMMENT, c.EXTRA
FROM information_schema.TABLES t
         LEFT JOIN information_schema.COLUMNS c
                   ON c.TABLE_CATALOG = t.TABLE_CATALOG
                       AND c.TABLE_SCHEMA = t.TABLE_SCHEMA
                       AND c.TABLE_NAME = t.TABLE_NAME
WHERE t.TABLE_SCHEMA = DATABASE()
ORDER BY c.TABLE_NAME ASC, c.ORDINAL_POSITION ASC`

	//nolint:lll
	// Query results look like:
	// +------------+----------+----------+-------------+----------+-----------+----------------+----------+---------+--------------------+-----------+-----------------+--------------+---------------------------+
	// |TABLE_SCHEMA|TABLE_NAME|TABLE_TYPE|TABLE_COMMENT|table_size|COLUMN_NAME|ORDINAL_POSITION|COLUMN_KEY|DATA_TYPE|COLUMN_TYPE         |IS_NULLABLE|COLUMN_DEFAULT   |COLUMN_COMMENT|EXTRA                      |
	// +------------+----------+----------+-------------+----------+-----------+----------------+----------+---------+--------------------+-----------+-----------------+--------------+---------------------------+
	// |sakila      |actor     |BASE TABLE|             |32768     |actor_id   |1               |PRI       |smallint |smallint(5) unsigned|NO         |NULL             |              |auto_increment             |
	// |sakila      |actor     |BASE TABLE|             |32768     |first_name |2               |          |varchar  |varchar(45)         |NO         |NULL             |              |                           |
	// |sakila      |actor     |BASE TABLE|             |32768     |last_name  |3               |MUL       |varchar  |varchar(45)         |NO         |NULL             |              |                           |
	// |sakila      |actor     |BASE TABLE|             |32768     |last_update|4               |          |timestamp|timestamp           |NO         |CURRENT_TIMESTAMP|              |on update CURRENT_TIMESTAMP|
	// |sakila      |actor_info|VIEW      |VIEW         |NULL      |actor_id   |1               |          |smallint |smallint(5) unsigned|NO         |0                |              |                           |

	var tblMetas []*source.TableMetadata
	var schema string
	var curTblName, curTblType, curTblComment sql.NullString
	var curTblSize sql.NullInt64
	var curTblMeta *source.TableMetadata

	// g is an errgroup for fetching the
	// row count for each table.
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(driver.Tuning.ErrgroupLimit)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	for rows.Next() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		var colName, colDefault, colNullable, colKey, colBaseType, colColumnType, colComment, colExtra sql.NullString
		var colPosition sql.NullInt64

		err = rows.Scan(&schema, &curTblName, &curTblType, &curTblComment, &curTblSize, &colName, &colPosition,
			&colKey, &colBaseType, &colColumnType, &colNullable, &colDefault, &colComment, &colExtra)
		if err != nil {
			return nil, errz.Err(err)
		}

		if !curTblName.Valid || !colName.Valid {
			// table may have been dropped during metadata collection
			log.Debug("Table not found during metadata collection")
			continue
		}

		if curTblMeta == nil || curTblMeta.Name != curTblName.String {
			// On our first time encountering a new table name, we create a new TableMetadata
			curTblMeta = &source.TableMetadata{
				Name:        curTblName.String,
				FQName:      schema + "." + curTblName.String,
				DBTableType: curTblType.String,
				TableType:   canonicalTableType(curTblType.String),
				Comment:     curTblComment.String,
			}

			if curTblSize.Valid {
				size := curTblSize.Int64
				curTblMeta.Size = &size
			}

			tblMetas = append(tblMetas, curTblMeta)

			rowCountTbl, rowCount, i := curTblName.String, &curTblMeta.RowCount, len(tblMetas)-1
			g.Go(func() error {
				gErr := db.QueryRowContext(gCtx, "SELECT COUNT(*) FROM `"+rowCountTbl+"`").Scan(rowCount)
				if gErr != nil {
					if hasErrCode(gErr, errNumTableNotExist) {
						// The table was probably dropped while we were collecting
						// metadata, but that's ok. We set the element to nil
						// and we'll filter it out later.
						log.Debug("Failed to get row count for table: ignoring error",
							lga.Table, curTblName.String,
							lga.Err, gErr)
						tblMetas[i] = nil
						return nil
					}

					return errz.Err(gErr)
				}
				return nil
			})
		}

		col := &source.ColMetadata{
			Name:         colName.String,
			Position:     colPosition.Int64,
			BaseType:     colBaseType.String,
			ColumnType:   colColumnType.String,
			DefaultValue: colDefault.String,
			Comment:      colComment.String,
		}

		col.Nullable, err = stringz.ParseBool(colNullable.String)
		if err != nil {
			return nil, err
		}

		col.Kind = kindFromDBTypeName(log, col.Name, col.BaseType)
		if strings.Contains(colKey.String, "PRI") {
			col.PrimaryKey = true
		}

		curTblMeta.Columns = append(curTblMeta.Columns, col)
	}

	err = g.Wait()
	if err != nil {
		return nil, err
	}

	err = rows.Err()
	if err != nil {
		return nil, errz.Err(err)
	}

	// tblMetas may contain nil elements if we failed to get the row
	// count for the table (which can happen if the table is dropped
	// during the metadata collection process). So we filter out any
	// nil elements.
	retTblMetas := make([]*source.TableMetadata, 0, len(tblMetas))
	for i := range tblMetas {
		if tblMetas[i] != nil {
			retTblMetas = append(retTblMetas, tblMetas[i])
		}
	}

	return retTblMetas, nil
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

			if destMeta[i].Kind() == kind.Text {
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
				if destMeta[i].Kind() == kind.Datetime {
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
func mungeSetDatetimeFromString(s string, i int, rec []any) {
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
func mungeSetZeroValue(i int, rec []any, destMeta sqlz.RecordMeta) {
	// REVISIT: do we need to do special handling for kind.Datetime
	//  and kind.Time (e.g. "00:00" for time)?
	z := reflect.Zero(destMeta[i].ScanType()).Interface()
	rec[i] = z
}

// canonicalTableType returns the canonical name for "BASE TABLE"
// and "VIEW".
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
