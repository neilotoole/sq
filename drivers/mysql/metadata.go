package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/lg/lgm"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// kindFromDBTypeName determines the Kind from the database
// type name. For example, "VARCHAR(64)" -> kind.Text.
func kindFromDBTypeName(ctx context.Context, colName, dbTypeName string) kind.Kind {
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
		lg.FromContext(ctx).Warn(
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

// setScanType ensures that ctd's scan type field is set appropriately.
func setScanType(ctd *record.ColumnTypeData, knd kind.Kind) {
	if knd == kind.Decimal {
		// Special handling for kind.Decimal, because MySQL doesn't natively
		// know how to handle it.
		ctd.ScanType = sqlz.RTypeNullDecimal
		return
	}
}

func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, error) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		knd := kindFromDBTypeName(ctx, colType.Name(), colType.DatabaseTypeName())
		colTypeData := record.NewColumnTypeData(colType, knd)
		setScanType(colTypeData, knd)
		sColTypeData[i] = colTypeData
		ogColNames[i] = colTypeData.Name
	}

	mungedColNames, err := driver.MungeResultColNames(ctx, ogColNames)
	if err != nil {
		return nil, err
	}

	recMeta := make(record.Meta, len(colTypes))
	for i := range sColTypeData {
		recMeta[i] = record.NewFieldMeta(sColTypeData[i], mungedColNames[i])
	}

	return recMeta, nil
}

// getNewRecordFunc returns a NewRecordFunc that, after interacting
// with the standard driver.NewRecordFromScanRow, munges any skipped fields.
// In particular sql.NullTime is unboxed to *time.Time, and TIME fields
// are munged from RawBytes to string.
func getNewRecordFunc(rowMeta record.Meta) driver.NewRecordFunc {
	return func(row []any) (record.Record, error) {
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
// implementation of Grip.TableMetadata.
func getTableMetadata(ctx context.Context, db sqlz.DB, tblName string) (*metadata.Table, error) {
	query := `SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_TYPE, TABLE_COMMENT, (DATA_LENGTH + INDEX_LENGTH) AS table_size,
(SELECT COUNT(*) FROM ` + "`" + tblName + "`" + `) AS row_count
FROM information_schema.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?`

	var schema string
	var tblSize sql.NullInt64
	tblMeta := &metadata.Table{}

	err := db.QueryRowContext(ctx, query, tblName).
		Scan(&schema, &tblMeta.Name, &tblMeta.DBTableType, &tblMeta.Comment, &tblSize, &tblMeta.RowCount)
	if err != nil {
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)
	progress.DebugDelay()

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
func getColumnMetadata(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.Column, error) {
	log := lg.FromContext(ctx)

	const query = `SELECT column_name, data_type, column_type, ordinal_position, column_default,
       is_nullable, column_key, column_comment, extra
FROM information_schema.columns cols
WHERE cols.TABLE_SCHEMA = DATABASE() AND cols.TABLE_NAME = ?
ORDER BY cols.ordinal_position ASC`

	rows, err := db.QueryContext(ctx, query, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	var cols []*metadata.Column

	for rows.Next() {
		col := &metadata.Column{}
		var isNullable, colKey, extra string

		defVal := &sql.NullString{}
		err = rows.Scan(&col.Name, &col.BaseType, &col.ColumnType, &col.Position, defVal, &isNullable, &colKey,
			&col.Comment, &extra)
		if err != nil {
			return nil, errw(err)
		}
		progress.Incr(ctx, 1)
		progress.DebugDelay()

		if strings.EqualFold("YES", isNullable) {
			col.Nullable = true
		}

		if strings.Contains(colKey, "PRI") {
			col.PrimaryKey = true
		}

		col.DefaultValue = defVal.String
		col.Kind = kindFromDBTypeName(ctx, col.Name, col.BaseType)

		cols = append(cols, col)
	}

	return cols, errw(rows.Err())
}

// getSourceMetadata is the implementation of driver.Grip.SourceMetadata.
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
func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB, noSchema bool) (*metadata.Source, error) {
	ctx = options.NewContext(ctx, src.Options)

	md := &metadata.Source{
		Driver:   Type,
		DBDriver: Type,
		Handle:   src.Handle,
		Location: src.Location,
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(driver.OptTuningErrgroupLimit.Get(src.Options))

	g.Go(func() error {
		return doRetry(gCtx, func() error {
			return setSourceSummaryMeta(gCtx, db, md)
		})
	})

	g.Go(func() error {
		return doRetry(gCtx, func() error {
			var err error
			md.DBProperties, err = getDBProperties(gCtx, db)
			return err
		})
	})

	if !noSchema {
		g.Go(func() error {
			return doRetry(gCtx, func() error {
				var err error
				md.Tables, err = getAllTblMetas(gCtx, db)
				return err
			})
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	for _, tbl := range md.Tables {
		if tbl.TableType == sqlz.TableTypeTable {
			md.TableCount++
		} else if tbl.TableType == sqlz.TableTypeView {
			md.ViewCount++
		}
	}

	return md, nil
}

func setSourceSummaryMeta(ctx context.Context, db sqlz.DB, md *metadata.Source) error {
	const summaryQuery = `SELECT @@GLOBAL.version, @@GLOBAL.version_comment, @@GLOBAL.version_compile_os,
       @@GLOBAL.version_compile_machine, DATABASE(), CURRENT_USER(),
       (SELECT CATALOG_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = DATABASE() LIMIT 1),
       (SELECT SUM( data_length + index_length )
        FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE()) AS size`

	var version, versionComment, versionOS, versionArch, schema string
	var size sql.NullInt64
	err := db.QueryRowContext(ctx, summaryQuery).Scan(
		&version,
		&versionComment,
		&versionOS,
		&versionArch,
		&schema,
		&md.User,
		&md.Catalog,
		&size,
	)
	if err != nil {
		return errw(err)
	}
	progress.Incr(ctx, 1)
	progress.DebugDelay()

	md.Name = schema
	md.Schema = schema
	md.FQName = md.Catalog + "." + schema
	if size.Valid {
		md.Size = size.Int64
	}
	md.DBVersion = version
	md.DBProduct = fmt.Sprintf("%s %s / %s (%s)", versionComment, version, versionOS, versionArch)
	return nil
}

// getDBProperties returns the db settings as observed via "SHOW VARIABLES".
func getDBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	log := lg.FromContext(ctx)

	rows, err := db.QueryContext(ctx, "SHOW VARIABLES")
	if err != nil {
		return nil, errw(err)
	}
	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	m := map[string]any{}
	for rows.Next() {
		var name string
		var val string

		err = rows.Scan(&name, &val)
		if err != nil {
			return nil, errw(err)
		}

		progress.Incr(ctx, 1)
		progress.DebugDelay()

		// Narrow setting to bool or int if possible.
		var (
			v any = val
			i int
			b bool
		)
		if i, err = strconv.Atoi(val); err == nil {
			v = i
		} else if b, err = stringz.ParseBool(val); err == nil {
			v = b
		}

		m[name] = v
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return m, nil
}

// getAllTblMetas returns Table for each table/view in db.
func getAllTblMetas(ctx context.Context, db sqlz.DB) ([]*metadata.Table, error) {
	log := lg.FromContext(ctx)

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

	var (
		tblMetas                              []*metadata.Table
		schema                                string
		curTblName, curTblType, curTblComment sql.NullString
		curTblSize                            sql.NullInt64
		curTblMeta                            *metadata.Table
	)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
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
			return nil, errw(err)
		}

		progress.Incr(ctx, 1)
		progress.DebugDelay()

		if !curTblName.Valid || !colName.Valid {
			// table may have been dropped during metadata collection
			log.Debug("Table not found during metadata collection")
			continue
		}

		if curTblMeta == nil || curTblMeta.Name != curTblName.String {
			// On our first time encountering a new table name, we create a new Table
			curTblMeta = &metadata.Table{
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
		}

		col := &metadata.Column{
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

		col.Kind = kindFromDBTypeName(ctx, col.Name, col.BaseType)
		if strings.Contains(colKey.String, "PRI") {
			col.PrimaryKey = true
		}

		curTblMeta.Columns = append(curTblMeta.Columns, col)
	}

	err = rows.Err()
	if err != nil {
		return nil, errw(err)
	}

	// tblMetas may contain nil elements if we failed to get the row
	// count for the table (which can happen if the table is dropped
	// during the metadata collection process). So we filter out any
	// nil elements.
	tblMetas = lo.Reject(tblMetas, func(item *metadata.Table, index int) bool {
		return item == nil
	})

	tblNames := make([]string, 0, len(tblMetas))
	for i := range tblMetas {
		if tblMetas[i] != nil {
			tblNames = append(tblNames, tblMetas[i].Name)
		}
	}

	// We still need to populate table row counts.
	var mTblRowCounts map[string]int64
	if mTblRowCounts, err = getTableRowCountsBatch(ctx, db, tblNames); err != nil {
		return nil, err
	}

	for i := range tblMetas {
		tblMetas[i].RowCount = mTblRowCounts[tblMetas[i].Name]
	}

	return tblMetas, nil
}

// getTableRowCountsBatch invokes getTableRowCounts, but batches tblNames
// into chunks, because MySQL will error if the query becomes too large.
func getTableRowCountsBatch(ctx context.Context, db sqlz.DB, tblNames []string) (map[string]int64, error) {
	// TODO: What value should batchSize be?
	const batchSize = 100
	var (
		all     = map[string]int64{}
		batches = lo.Chunk(tblNames, batchSize)
	)

	for i := range batches {
		got, err := getTableRowCounts(ctx, db, batches[i])
		if err != nil {
			return nil, err
		}

		for k, v := range got {
			all[k] = v
		}
	}

	return all, nil
}

// getTableRowCounts returns a map of table name to count of rows
// in that table, as returned by "SELECT COUNT(*) FROM tblName".
// If a table does not exist, the error is logged, but then
// disregarded, as it's possible that a table can be deleted during
// the operation. Thus the length of the returned map may be less
// than len(tblNames).
func getTableRowCounts(ctx context.Context, db sqlz.DB, tblNames []string) (map[string]int64, error) {
	log := lg.FromContext(ctx)

	var rows *sql.Rows
	var err error

	for {
		if len(tblNames) == 0 {
			return map[string]int64{}, nil
		}

		var qb strings.Builder
		for i := 0; i < len(tblNames); i++ {
			if i > 0 {
				qb.WriteString("\nUNION\n")
			}
			qb.WriteString(fmt.Sprintf("SELECT %s AS tn, COUNT(*) AS rc FROM %s",
				stringz.SingleQuote(tblNames[i]), stringz.BacktickQuote(tblNames[i])))
		}

		query := qb.String()
		rows, err = db.QueryContext(ctx, query)
		if err == nil {
			// The query is good, continue below.
			break
		}

		err = errw(err)
		if errz.Has[*driver.NotExistError](err) {
			// Sometimes a table can get deleted during the operation. If so,
			// we just remove that table from the list, and try again.
			// We could also do this entire thing in a transaction, but where's
			// the fun in that...
			schema, tblName, ok := extractTblNameFromNotExistErr(err)
			if ok && tblName != "" {
				log.Warn("Table doesn't exist, will try again without that table",
					lga.Schema, schema, lga.Table, tblName)
				tblNames = lo.Without(tblNames, tblName)
				continue
			}
		}
		return nil, err
	}

	defer lg.WarnIfCloseError(log, lgm.CloseDBRows, rows)

	var (
		m       = make(map[string]int64, len(tblNames))
		count   int64
		tblName string
	)
	for rows.Next() {
		if err = rows.Scan(&tblName, &count); err != nil {
			return nil, errw(err)
		}
		progress.Incr(ctx, 1)
		progress.DebugDelay()

		m[tblName] = count
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return m, nil
}

// extractTblNameFromNotExistErr is a filthy hack to extract schema
// and table from err 1146 (table doesn't exist).
func extractTblNameFromNotExistErr(err error) (schema, table string, ok bool) {
	if err == nil {
		return "", "", false
	}

	var e *mysql.MySQLError
	if !errors.As(err, &e) {
		return "", "", false
	}

	if e.Number != errNumTableNotExist {
		return "", "", false
	}

	s := strings.TrimSuffix(strings.TrimPrefix(e.Message, "Table '"), "' doesn't exist")

	if s == "" {
		return "", "", false
	}

	if schema, table, ok = strings.Cut(s, "."); ok {
		return schema, table, ok
	}

	return "", "", false
}

// newInsertMungeFunc is lifted from driver.DefaultInsertMungeFunc.
func newInsertMungeFunc(destTbl string, destMeta record.Meta) driver.InsertMungeFunc {
	return func(rec record.Record) error {
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
func mungeSetZeroValue(i int, rec []any, destMeta record.Meta) {
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
