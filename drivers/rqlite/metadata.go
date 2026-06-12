package rqlite

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// recordMetaFromColumnTypes builds record.Meta for colTypes returned by
// gorqlite. The shape matches the sqlite3 driver's helper: the SQL is
// SQLite's, so the column-type names and affinity rules apply verbatim.
func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, error) {
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		dbTypeName := colType.DatabaseTypeName()
		knd := kindFromDBTypeName(ctx, colType.Name(), dbTypeName, colType.ScanType())
		colTypeData := record.NewColumnTypeData(colType, knd)
		setScanType(ctx, colTypeData)
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

// setScanType normalizes colType.ScanType to the appropriate
// sql.NullTYPE (or rtypeNullTime, for time-kind columns). If the
// driver-supplied scan type is nil — which gorqlite produces for some
// column shapes — the destination is chosen from colType.Kind alone.
func setScanType(ctx context.Context, colType *record.ColumnTypeData) {
	scanType, knd := colType.ScanType, colType.Kind

	if scanType != nil {
		switch scanType {
		default:
			colType.ScanType = sqlz.RTypeAny
		case sqlz.RTypeInt64:
			colType.ScanType = sqlz.RTypeNullInt64
		case sqlz.RTypeFloat64:
			colType.ScanType = sqlz.RTypeNullFloat64
		case sqlz.RTypeString:
			colType.ScanType = sqlz.RTypeNullString
		case sqlz.RTypeBool:
			colType.ScanType = sqlz.RTypeNullBool
		case sqlz.RTypeTime:
			colType.ScanType = sqlz.RTypeNullTime
		case sqlz.RTypeBytes:
			// no change.
		}
	}

	switch knd { //nolint:exhaustive
	default:
		lg.FromContext(ctx).Warn("Unknown kind for col",
			lga.Col, colType.Name,
			lga.DBType, colType.DatabaseTypeName,
		)
		scanType = sqlz.RTypeAny
	case kind.Text:
		scanType = sqlz.RTypeNullString
	case kind.Decimal:
		scanType = sqlz.RTypeNullDecimal
	case kind.Int:
		scanType = sqlz.RTypeNullInt64
	case kind.Bool:
		scanType = sqlz.RTypeNullBool
	case kind.Float:
		scanType = sqlz.RTypeNullFloat64
	case kind.Bytes:
		scanType = sqlz.RTypeBytes
	case kind.Datetime:
		scanType = rtypeNullTime
	case kind.Date:
		scanType = rtypeNullTime
	case kind.Time:
		scanType = sqlz.RTypeNullString
	}

	colType.ScanType = scanType
}

// kindFromDBTypeName resolves a SQLite-affinity column type to its
// kind.Kind. The rules are SQLite's, so the implementation is a copy
// of the sqlite3 driver's. See
// https://www.sqlite.org/datatype3.html#determination_of_column_affinity.
//
// dbTypeName may be empty (e.g. for COUNT(*)); scanType, when non-nil,
// is used to break the tie.
func kindFromDBTypeName(ctx context.Context, colName, dbTypeName string, scanType reflect.Type) kind.Kind {
	log := lg.FromContext(ctx)
	if dbTypeName == "" {
		if scanType == nil {
			// Per the SQLite docs, a column with no declared type has
			// affinity BLOB; that's the safest fallback when we have
			// no scan-type hint either.
			return kind.Bytes
		}

		switch scanType {
		default:
			return kind.Unknown
		case sqlz.RTypeInt64:
			return kind.Int
		case sqlz.RTypeFloat64:
			return kind.Float
		case sqlz.RTypeString:
			return kind.Text
		case sqlz.RTypeBytes:
			return kind.Bytes
		}
	}

	var knd kind.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	// Strip any parameterized suffix (e.g. "VARCHAR(255)" -> "VARCHAR").
	if i := strings.IndexRune(dbTypeName, '('); i > 0 {
		dbTypeName = dbTypeName[0:i]
	}

	switch dbTypeName {
	case "INT", "INTEGER", "TINYINT", "SMALLINT", "MEDIUMINT", "BIGINT", "UNSIGNED BIG INT", "INT2", "INT8":
		knd = kind.Int
	case "REAL", "DOUBLE", "DOUBLE PRECISION", "FLOAT":
		knd = kind.Float
	case "DECIMAL":
		knd = kind.Decimal
	case "TEXT", "CHARACTER", "VARCHAR", "VARYING CHARACTER", "NCHAR", "NATIVE CHARACTER", "NVARCHAR", "CLOB":
		knd = kind.Text
	case "BLOB":
		knd = kind.Bytes
	case "DATETIME", "TIMESTAMP":
		knd = kind.Datetime
	case "DATE":
		knd = kind.Date
	case "TIME":
		knd = kind.Time
	case "BOOLEAN":
		knd = kind.Bool
	case "NUMERIC":
		// NUMERIC could be int, float, or big decimal; Decimal is the
		// safest sink because it accepts any numeric value.
		knd = kind.Decimal
	}

	if knd != kind.Unknown {
		return knd
	}

	// Fall back to SQLite affinity rules. SQLite's default affinity is
	// NUMERIC; sq surfaces that as kind.Text.
	switch {
	default:
		knd = kind.Unknown
		log.Warn("Unknown SQLite database column type: using alt",
			lga.DBType, dbTypeName,
			lga.Col, colName,
			lga.Kind, knd,
		)
	case strings.Contains(dbTypeName, "INT"):
		knd = kind.Int
	case strings.Contains(dbTypeName, "TEXT"),
		strings.Contains(dbTypeName, "CHAR"),
		strings.Contains(dbTypeName, "CLOB"):
		knd = kind.Text
	case strings.Contains(dbTypeName, "BLOB"):
		knd = kind.Bytes
	case strings.Contains(dbTypeName, "REAL"),
		strings.Contains(dbTypeName, "FLOA"),
		strings.Contains(dbTypeName, "DOUB"):
		knd = kind.Float
	}

	return knd
}

// DBTypeForKind returns the SQLite database type for kind.
func DBTypeForKind(knd kind.Kind) string {
	switch knd {
	default:
		panic(fmt.Sprintf("unknown kind {%s}", knd))
	case kind.Text, kind.Null, kind.Unknown:
		return "TEXT"
	case kind.Int:
		return "INTEGER"
	case kind.Float:
		return "REAL"
	case kind.Bytes:
		return "BLOB"
	case kind.Decimal:
		return "NUMERIC"
	case kind.Bool:
		return "BOOLEAN"
	case kind.Datetime:
		return "DATETIME"
	case kind.Date:
		return "DATE"
	case kind.Time:
		return "TIME"
	}
}

// newRecordFromScanRow converts a Scan row into a record.Record. The
// shape mirrors the sqlite3 driver's helper. Two rqlite-specific
// quirks are handled here:
//
//   - gorqlite hands back JSON numbers as float64, so for kind.Int
//     columns the float64 is coerced to int64. For kind.Decimal
//     columns the value is shaped as decimal.Decimal, with a
//     whole-number coercion to int64 that mirrors SQLite's NUMERIC
//     affinity (mattn/go-sqlite3 surfaces integer-valued NUMERIC
//     cells as int64; we match that so the cross-driver record
//     contract is honored).
//   - The scan destination for kind.Decimal columns is
//     *decimal.NullDecimal (set in setScanType), so that case is
//     unwrapped here too.
//
//nolint:funlen,gocognit
func newRecordFromScanRow(meta record.Meta, row []any) (rec record.Record) {
	rec = make([]any, len(row))

	for i := 0; i < len(row); i++ {
		if row[i] == nil {
			rec[i] = nil
			continue
		}

		col := row[i]
		if ptr, ok := col.(*any); ok {
			col = *ptr
		}

		switch col := col.(type) {
		default:
			rec[i] = col
			continue
		case nil:
			rec[i] = nil
		case *int64:
			record.SetKindIfUnknown(meta, i, kind.Int)
			rec[i] = *col
		case int64:
			record.SetKindIfUnknown(meta, i, kind.Int)
			rec[i] = col
		case *float64:
			rec[i] = coerceFloat64(meta, i, *col)
		case float64:
			rec[i] = coerceFloat64(meta, i, col)
		case *bool:
			record.SetKindIfUnknown(meta, i, kind.Bool)
			rec[i] = *col
		case bool:
			record.SetKindIfUnknown(meta, i, kind.Bool)
			rec[i] = col
		case *string:
			record.SetKindIfUnknown(meta, i, kind.Text)
			rec[i] = *col
		case string:
			record.SetKindIfUnknown(meta, i, kind.Text)
			rec[i] = col
		case *[]byte:
			if col == nil || *col == nil {
				rec[i] = nil
				continue
			}
			if meta[i].Kind() != kind.Bytes {
				rec[i] = string(*col)
				record.SetKindIfUnknown(meta, i, kind.Text)
				continue
			}
			if len(*col) == 0 {
				rec[i] = []byte{}
			} else {
				dest := make([]byte, len(*col))
				copy(dest, *col)
				rec[i] = dest
			}
			record.SetKindIfUnknown(meta, i, kind.Bytes)
		case *sql.NullInt64:
			if col.Valid {
				rec[i] = col.Int64
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Int)
		case *sql.NullString:
			if col.Valid {
				rec[i] = col.String
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Text)
		case *sql.RawBytes:
			if col == nil || *col == nil {
				rec[i] = nil
				continue
			}
			knd := meta[i].Kind()
			if len(*col) == 0 {
				if knd == kind.Bytes {
					rec[i] = []byte{}
				} else {
					var s string
					rec[i] = s
					record.SetKindIfUnknown(meta, i, kind.Text)
				}
				continue
			}
			dest := make([]byte, len(*col))
			copy(dest, *col)
			if knd == kind.Bytes {
				rec[i] = dest
			} else {
				rec[i] = string(dest)
				record.SetKindIfUnknown(meta, i, kind.Text)
			}
		case *sql.NullFloat64:
			if col.Valid {
				rec[i] = col.Float64
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Float)
		case *decimal.NullDecimal:
			if !col.Valid {
				rec[i] = nil
			} else {
				rec[i] = coerceDecimal(col.Decimal)
			}
			record.SetKindIfUnknown(meta, i, kind.Decimal)
		case *decimal.Decimal:
			rec[i] = coerceDecimal(*col)
			record.SetKindIfUnknown(meta, i, kind.Decimal)
		case decimal.Decimal:
			rec[i] = coerceDecimal(col)
			record.SetKindIfUnknown(meta, i, kind.Decimal)
		case *sql.NullBool:
			if col.Valid {
				rec[i] = col.Bool
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Bool)
		case *sql.NullTime:
			if col.Valid {
				rec[i] = col.Time
			} else {
				rec[i] = nil
			}
			record.SetKindIfUnknown(meta, i, kind.Datetime)
		case *nullTime:
			// No SetKindIfUnknown call here: *nullTime is only allocated
			// for columns already classified as kind.Datetime/Date, so
			// the kind is never unknown at this point.
			switch {
			case !col.Valid:
				rec[i] = nil
			case col.IsTime:
				rec[i] = col.Time
			default:
				rec[i] = col.String
			}
		case *time.Time:
			rec[i] = *col
			record.SetKindIfUnknown(meta, i, kind.Datetime)
		case time.Time:
			rec[i] = col
			record.SetKindIfUnknown(meta, i, kind.Datetime)
		}
	}

	return rec
}

// coerceFloat64 reshapes a float64 to the type implied by the
// column's kind. gorqlite returns every JSON number as float64 so we
// have to demote back to int64 for integer columns; otherwise the
// cross-driver record contract (int columns yield int64) is broken.
// For Decimal columns the value is converted to decimal.Decimal,
// with an additional whole-number coercion to int64 that matches
// what mattn/go-sqlite3 emits natively for NUMERIC-affinity columns
// that happen to hold integers (Sakila's actor_id is exactly this
// shape).
//
// Unknown kinds pass through as float64 and have the kind set to
// Float, mirroring the original behavior.
func coerceFloat64(meta record.Meta, i int, v float64) any {
	switch meta[i].Kind() { //nolint:exhaustive
	case kind.Int:
		return int64(v)
	case kind.Bool:
		return v != 0
	case kind.Decimal:
		return coerceDecimal(decimal.NewFromFloat(v))
	default:
		record.SetKindIfUnknown(meta, i, kind.Float)
		return v
	}
}

// coerceDecimal demotes whole-number decimals to int64 so NUMERIC
// columns whose stored values are integers match the cross-driver
// record contract. Non-integer decimals are returned as-is.
func coerceDecimal(d decimal.Decimal) any {
	if d.IsInteger() {
		return d.IntPart()
	}
	return d
}

// getTableMetadata returns metadata for a single table. The shape
// mirrors the sqlite3 driver's helper. Virtual-table BaseType
// recovery via SELECT typeof() is omitted; rqlite users are
// unlikely to be hitting FTS5/r-tree tables.
func getTableMetadata(ctx context.Context, db sqlz.DB, tblName string) (*metadata.Table, error) {
	log := lg.FromContext(ctx)
	tblMeta := &metadata.Table{Name: tblName}

	// The table name is bound as a query parameter where it appears in
	// string-literal position: interpolating it with Go's %q resolved a
	// double-quoted token as an identifier first, so a table named after
	// a sqlite_master column (name, type, sql) produced a tautology, and
	// a name containing a double quote got Go backslash escaping, which
	// SQLite rejects (gh777). In identifier position (FROM),
	// stringz.DoubleQuote applies proper SQLite quoting, doubling any
	// embedded double quote.
	const tpl = `SELECT
(SELECT COUNT(*) FROM %s),
(SELECT type FROM sqlite_master WHERE name = ? LIMIT 1),
(SELECT 1 FROM sqlite_master WHERE name = ? AND substr("sql",0,21) == 'CREATE VIRTUAL TABLE') AS is_virtual,
(SELECT name FROM pragma_database_list ORDER BY seq LIMIT 1)`

	var schemaName string
	// is_virtual is read as NullFloat64 because rqlite returns
	// integer SQL literals as JSON numbers, which can't scan into
	// sql.NullBool. NULL means not a virtual table; any non-zero
	// value means virtual.
	var isVirtualTbl sql.NullFloat64
	query := fmt.Sprintf(tpl, stringz.DoubleQuote(tblMeta.Name))
	err := db.QueryRowContext(ctx, query, tblMeta.Name, tblMeta.Name).Scan(
		&tblMeta.RowCount, &tblMeta.DBTableType, &isVirtualTbl, &schemaName)
	if err != nil {
		return nil, errw(err)
	}

	switch {
	case isVirtualTbl.Valid && isVirtualTbl.Float64 != 0:
		tblMeta.TableType = sqlz.TableTypeVirtual
	case tblMeta.DBTableType == sqlz.TableTypeView:
		tblMeta.TableType = sqlz.TableTypeView
	case tblMeta.DBTableType == sqlz.TableTypeTable:
		tblMeta.TableType = sqlz.TableTypeTable
	default:
	}

	tblMeta.FQName = schemaName + "." + tblName

	query = "PRAGMA TABLE_INFO(" + stringz.DoubleQuote(tblMeta.Name) + ")"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	for rows.Next() {
		col := &metadata.Column{}
		var notnull int64
		defaultValue := &sql.NullString{}
		pkValue := &sql.NullInt64{}
		if err = rows.Scan(
			&col.Position, &col.Name, &col.BaseType, &notnull, defaultValue, pkValue,
		); err != nil {
			return nil, errw(err)
		}
		col.PrimaryKey = pkValue.Int64 > 0
		col.ColumnType = col.BaseType
		col.Nullable = notnull == 0
		col.DefaultValue = defaultValue.String
		col.Kind = kindFromDBTypeName(ctx, col.Name, col.BaseType, nil)
		tblMeta.Columns = append(tblMeta.Columns, col)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	if tblMeta.TableType != sqlz.TableTypeTable {
		return tblMeta, nil
	}

	outgoing, fkErr := getTableForeignKeys(ctx, db, tblName)
	if fkErr != nil {
		return nil, errz.Wrapf(fkErr, "rqlite: failed to read foreign keys for {%s}", tblName)
	}
	if len(outgoing) > 0 {
		tblMeta.FK = metadata.NewFKGroup(outgoing, nil)
	}

	return tblMeta, nil
}

// getTableForeignKeys returns the outgoing foreign-key constraints
// declared on tblName, using SQLite's pragma_foreign_key_list. Returns
// nil if the table has no foreign keys. Composite foreign keys are
// returned as a single ForeignKey whose Columns/RefColumns slices are
// populated in the order returned by pragma_foreign_key_list (ORDER BY id, seq).
func getTableForeignKeys(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)
	// pragma_foreign_key_list returns columns:
	//   id, seq, table, from, to, on_update, on_delete, match
	// One row per (constraint, column-pair). id groups composite FKs.
	const q = `SELECT id, seq, "table", "from", "to", on_update, on_delete
FROM pragma_foreign_key_list(?)
ORDER BY id, seq`

	rows, err := db.QueryContext(ctx, q, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var (
		fks   []*metadata.ForeignKey
		curID int64 = -1
		curFK *metadata.ForeignKey
	)
	for rows.Next() {
		var (
			id, seq                  int64
			refTable, fromCol, toCol string
			onUpdate, onDelete       sql.NullString
		)
		if err = rows.Scan(&id, &seq, &refTable, &fromCol, &toCol, &onUpdate, &onDelete); err != nil {
			return nil, errw(err)
		}

		if curFK == nil || id != curID {
			curID = id
			curFK = &metadata.ForeignKey{
				Table:    tblName,
				RefTable: refTable,
				OnDelete: onDelete.String,
				OnUpdate: onUpdate.String,
			}
			fks = append(fks, curFK)
		}
		curFK.Columns = append(curFK.Columns, fromCol)
		curFK.RefColumns = append(curFK.RefColumns, toCol)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return fks, nil
}

// getAllTableMetadata returns metadata for every table in db's schema.
// Unlike getTableMetadata, this bulk path does not populate per-table
// foreign keys; callers that need full FK details should re-fetch
// individual tables via getTableMetadata.
func getAllTableMetadata(ctx context.Context, db sqlz.DB, schemaName string) ([]*metadata.Table, error) {
	log := lg.FromContext(ctx)

	const query = `
SELECT m.name as table_name, m.type, p.cid, p.name, p.type, p.'notnull' as 'notnull', p.dflt_value, p.pk,
(substr(m.sql, 0, 21) == 'CREATE VIRTUAL TABLE') AS is_virtual
FROM sqlite_master AS m JOIN pragma_table_info(m.name) AS p
ORDER BY m.name, p.cid
`

	var (
		tblMetas []*metadata.Table
		tblNames []string
		curTblName,
		curTblType string
		// is_virtual is a CASE expression, not a typed column. rqlite
		// returns it as a JSON number so we read it as float64 and
		// treat any non-zero value as virtual.
		curTblIsVirtual float64
		curTblMeta      *metadata.Table
	)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	for rows.Next() {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}

		col := &metadata.Column{}
		var notnull int64
		colDefault := &sql.NullString{}
		pkValue := &sql.NullInt64{}

		if err = rows.Scan(
			&curTblName, &curTblType, &col.Position, &col.Name, &col.BaseType,
			&notnull, colDefault, pkValue, &curTblIsVirtual,
		); err != nil {
			return nil, errw(err)
		}

		if strings.HasPrefix(curTblName, "sqlite_") {
			// Skip system tables such as sqlite_sequence.
			continue
		}

		if curTblMeta == nil || curTblMeta.Name != curTblName {
			curTblMeta = &metadata.Table{
				Name:        curTblName,
				FQName:      schemaName + "." + curTblName,
				DBTableType: curTblType,
			}
			switch {
			case curTblIsVirtual != 0:
				curTblMeta.TableType = sqlz.TableTypeVirtual
			case curTblMeta.DBTableType == sqlz.TableTypeView:
				curTblMeta.TableType = sqlz.TableTypeView
			case curTblMeta.DBTableType == sqlz.TableTypeTable:
				curTblMeta.TableType = sqlz.TableTypeTable
			default:
			}
			tblNames = append(tblNames, curTblName)
			tblMetas = append(tblMetas, curTblMeta)
		}

		col.PrimaryKey = pkValue.Int64 > 0
		col.ColumnType = col.BaseType
		col.Nullable = notnull == 0
		col.DefaultValue = colDefault.String
		col.Kind = kindFromDBTypeName(ctx, col.Name, col.BaseType, nil)
		curTblMeta.Columns = append(curTblMeta.Columns, col)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	rowCounts, err := getTblRowCounts(ctx, db, tblNames)
	if err != nil {
		return nil, errw(err)
	}
	for i := range rowCounts {
		tblMetas[i].RowCount = rowCounts[i]
	}

	return tblMetas, nil
}

// getTblRowCounts returns the row count of each named table in a
// single round-trip, using the union-based query the sqlite3 driver
// settled on after benchmarking. SQLITE_MAX_COMPOUND_SELECT caps each
// query at 500 SELECT terms, so we batch.
//
// If a table named in tblNames is dropped by a concurrent writer
// between the enumerate step in getAllTableMetadata and the COUNT
// batch here, the UNION ALL fails with "no such table:". We fall
// back to per-table COUNTs for that batch and record -1 for any
// table that has since vanished, so callers can detect (or skip).
func getTblRowCounts(ctx context.Context, db sqlz.DB, tblNames []string) ([]int64, error) {
	log := lg.FromContext(ctx)
	const maxCompoundSelect = 500

	tblCounts := make([]int64, len(tblNames))
	var (
		sb    strings.Builder
		terms int
		j     int
	)

	for i := 0; i < len(tblNames); i++ {
		if terms > 0 {
			sb.WriteString(" UNION ALL ")
		}
		sb.WriteString("SELECT COUNT(*) FROM " + stringz.DoubleQuote(tblNames[i]))
		terms++

		if terms != maxCompoundSelect && i != len(tblNames)-1 {
			continue
		}

		rows, err := db.QueryContext(ctx, sb.String())
		if err != nil {
			wrapped := errw(err)
			if errz.Has[*driver.NotExistError](wrapped) {
				// A table enumerated by sqlite_master was dropped
				// before we could COUNT it. Fall back to per-table
				// COUNTs across the current batch, recording -1 for
				// any name that has since vanished.
				batchEnd := j + terms
				if err = countTblsIndividually(ctx, db, tblNames[j:batchEnd], tblCounts[j:batchEnd]); err != nil {
					return nil, err
				}
				j = batchEnd
				terms = 0
				sb.Reset()
				continue
			}
			return nil, wrapped
		}
		for rows.Next() {
			if err = rows.Scan(&tblCounts[j]); err != nil {
				sqlz.CloseRows(log, rows)
				return nil, errw(err)
			}
			j++
		}
		if err = rows.Err(); err != nil {
			sqlz.CloseRows(log, rows)
			return nil, errw(err)
		}
		if err = rows.Close(); err != nil {
			return nil, errw(err)
		}

		terms = 0
		sb.Reset()
	}

	return tblCounts, nil
}

// countTblsIndividually issues a per-table SELECT COUNT(*) for each
// name in names, writing the result to the matching slot in counts.
// Tables that have vanished (NotExistError) are recorded as -1; any
// other error aborts. This is the fallback path used by
// getTblRowCounts when the UNION ALL batch fails because of a
// concurrent DROP TABLE.
func countTblsIndividually(ctx context.Context, db sqlz.DB, names []string, counts []int64) error {
	for i, name := range names {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		var count int64
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM "+stringz.DoubleQuote(name)).Scan(&count)
		if err != nil {
			if errz.Has[*driver.NotExistError](errw(err)) {
				counts[i] = -1
				continue
			}
			return errw(err)
		}
		counts[i] = count
	}
	return nil
}

// getSourceMetadata builds the Source-level metadata for grip.
// Schema/version come from a small composite query; tables come from
// getAllTableMetadata (or are skipped when noSchema is true).
func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB, noSchema bool,
) (*metadata.Source, error) {
	md := &metadata.Source{
		Handle:   src.Handle,
		Driver:   drivertype.Rqlite,
		DBDriver: dbDrvr,
		Location: src.Location,
		Catalog:  "default",
	}

	const q = `SELECT sqlite_version(), (SELECT name FROM pragma_database_list ORDER BY seq LIMIT 1)`
	if err := db.QueryRowContext(ctx, q).Scan(&md.DBVersion, &md.Schema); err != nil {
		return nil, errw(err)
	}
	md.DBProduct = "rqlite (SQLite " + md.DBVersion + ")"
	md.Name = md.Schema
	md.FQName = md.Schema

	if noSchema {
		return md, nil
	}

	var err error
	md.Tables, err = getAllTableMetadata(ctx, db, md.Schema)
	if err != nil {
		return nil, err
	}
	for _, tbl := range md.Tables {
		switch tbl.TableType {
		case sqlz.TableTypeTable:
			md.TableCount++
		case sqlz.TableTypeView:
			md.ViewCount++
		}
	}

	return md, nil
}
