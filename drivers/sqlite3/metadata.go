package sqlite3

import "C"
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"golang.org/x/mod/semver"

	"github.com/neilotoole/sq/drivers/sqlite3/sqlparser"
	"github.com/neilotoole/sq/libsq/core/debugz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// recordMetaFromColumnTypes returns record.Meta for colTypes.
func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType, kindHints map[int]kind.Kind,
) (record.Meta, error) {
	// kindHints carries forced result-column kinds recorded during rendering
	// (e.g. sum() pinned to kind.Decimal). SQLite reports no usable type for
	// such expressions, so without a hint they would be surfaced as int/float
	// from the scanned value. See issue #839.
	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		// sqlite is very forgiving at times, e.g. execute
		// a query with a non-existent column name.
		// This can manifest as an empty db type name. This also
		// happens for functions such as COUNT(*).
		dbTypeName := colType.DatabaseTypeName()

		knd := kindFromDBTypeName(ctx, colType.Name(), dbTypeName, colType.ScanType())
		if hint, ok := kindHints[i]; ok {
			// Force the renderer-pinned kind. setScanType derives the scan
			// target from the kind, so kind.Decimal yields a decimal scan.
			knd = hint
		}
		colTypeData := record.NewColumnTypeData(colType, knd)

		// It's necessary to explicitly set the scan type because
		// the backing driver doesn't set it for whatever reason.
		setScanType(ctx, colTypeData) // REVISIT: Legacy? Do we still need this?

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

// setScanType ensures colTypeData.ScanType is set appropriately.
// If the scan type is nil, a scan type will be set based upon
// the col kind. The scan type can be nil in the case where rows.ColumnTypes
// was invoked before rows.Next (this is necessary for an empty table).
//
// If the scan type is NOT a sql.NullTYPE, the corresponding sql.NullTYPE will
// be set.
func setScanType(ctx context.Context, colType *record.ColumnTypeData) {
	scanType, knd := colType.ScanType, colType.Kind

	if scanType != nil {
		// If the scan type is already set, ensure it's sql.NullTYPE.
		switch scanType {
		default:
			// If it's not one of these types, we use "any".
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
			// no need to change if it's []byte
		}
	}

	switch knd { //nolint:exhaustive
	default:
		// Shouldn't happen?
		lg.FromContext(ctx).Warn(
			"Unknown kind for col",
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

// kindFromDBTypeName determines the kind.Kind from the database
// type name. For example, "VARCHAR(64)" -> kind.Text.
//
// See: https://www.sqlite.org/datatype3.html#determination_of_column_affinity
//
// The scanType arg may be nil (it may not be available to the caller). When
// non-nil it may be used to determine ambiguous cases. For example, dbTypeName
// is empty string for "COUNT(*)".
func kindFromDBTypeName(ctx context.Context, colName, dbTypeName string, scanType reflect.Type) kind.Kind {
	log := lg.FromContext(ctx)
	if dbTypeName == "" {
		// dbTypeName can be empty for functions such as COUNT() etc.
		// But we can infer the type from scanType (if non-nil).
		if scanType == nil {
			// According to the SQLite3 docs:
			//
			//   3. If the declared type for a column contains the
			//      string "BLOB" or **if no type is specified** then the
			//      column has affinity BLOB.
			//
			// However, I'm not certain how significant that claim is. It
			// might be more appropriate to return kind.Unknown here.
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

	// See the examples of type names in the sqlite docs linked above.
	// Given variations such as VARCHAR(255), we first trim the parens
	// parts. Thus VARCHAR(255) becomes VARCHAR.
	i := strings.IndexRune(dbTypeName, '(')
	if i > 0 {
		dbTypeName = dbTypeName[0:i]
	}

	// Try direct matches against common type names
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
		// NUMERIC is problematic. It could be an int, float, big decimal, etc.
		// kind.Decimal is safest as it can accept any numeric value.
		knd = kind.Decimal
	}

	// If we have a match, return now.
	if knd != kind.Unknown {
		return knd
	}

	// We didn't find an exact match, we'll use the Affinity rules
	// per the SQLite link provided earlier, noting that we default
	// to kind.Text (the docs specify default affinity NUMERIC, which
	// sq handles as kind.Text).
	switch {
	default:
		knd = kind.Unknown
		log.Warn(
			"Unknown SQLite database column type: using alt",
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

// DBTypeForKind returns the database type for kind.
// For example: Int --> INTEGER.
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

// getTableMetadata returns metadata for tblName in db.
func getTableMetadata(ctx context.Context, db sqlz.DB, tblName string) (*metadata.Table, error) {
	log := lg.FromContext(ctx)
	tblMeta := &metadata.Table{Name: tblName}
	// Note that there's no easy way of getting the physical size of
	// a table, so tblMeta.Size remains nil.

	// But we can get the row count and table type ("table" or "view").
	// The table name is bound as a query parameter where it appears in
	// string-literal position: interpolating it with Go's %q resolved a
	// double-quoted token as an identifier first, so a table named after
	// a sqlite_master column (name, type, sql) produced a tautology, and
	// a name containing a double quote got Go backslash escaping, which
	// SQLite rejects (gh777). In identifier position (FROM),
	// stringz.DoubleQuote applies proper SQLite quoting, doubling any
	// embedded double quote.
	// The type filter matters: a trigger (or index) may share a table's
	// name in sqlite_master, and without the filter the first matching
	// row could misreport the table's type.
	const tpl = `SELECT
(SELECT COUNT(*) FROM %s),
(SELECT type FROM sqlite_master WHERE name = ? AND type IN ('table','view') LIMIT 1),
(SELECT 1 FROM sqlite_master WHERE name = ?
 AND type = 'table' AND substr("sql",0,21) == 'CREATE VIRTUAL TABLE') AS is_virtual,
(SELECT name FROM pragma_database_list ORDER BY seq LIMIT 1)`

	var schema string
	var isVirtualTbl sql.NullBool
	query := fmt.Sprintf(tpl, stringz.DoubleQuote(tblMeta.Name))
	err := db.QueryRowContext(ctx, query, tblMeta.Name, tblMeta.Name).
		Scan(&tblMeta.RowCount, &tblMeta.DBTableType, &isVirtualTbl, &schema)
	if err != nil {
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)

	switch {
	case isVirtualTbl.Valid && isVirtualTbl.Bool:
		tblMeta.TableType = sqlz.TableTypeVirtual
	case tblMeta.DBTableType == sqlz.TableTypeView:
		tblMeta.TableType = sqlz.TableTypeView
	case tblMeta.DBTableType == sqlz.TableTypeTable:
		tblMeta.TableType = sqlz.TableTypeTable
	default:
	}

	tblMeta.FQName = schema + "." + tblName

	// cid	name		type		notnull	dflt_value	pk	hidden
	// 0	actor_id	INT			1		<null>		1	0
	// 1	film_id		INT			1		<null>		2	0
	// 2	last_update	TIMESTAMP	1		<null>		0	0
	// pragma_table_xinfo is used (rather than table_info) because it also
	// reports generated columns, which table_info omits entirely; its
	// hidden field is 0 for an ordinary column, 1 for a virtual-table
	// hidden column (skipped, to match table_info's user-visible set),
	// 2 for a VIRTUAL generated column, and 3 for a STORED one.
	// The table-valued pragma function takes the table name as a bound
	// parameter, eliminating an interpolated quoting site (gh777).
	query = `SELECT cid, name, type, "notnull", dflt_value, pk, hidden FROM pragma_table_xinfo(?)`
	rows, err := db.QueryContext(ctx, query, tblMeta.Name)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		col := &metadata.Column{}
		var notnull, hidden int64
		defaultValue := &sql.NullString{}
		pkValue := &sql.NullInt64{}
		err = rows.Scan(&col.Position, &col.Name, &col.BaseType, &notnull, defaultValue, pkValue, &hidden)
		if err != nil {
			return nil, errw(err)
		}

		if hidden == 1 {
			// Virtual-table hidden column: excluded from inspect to match
			// the user-visible column set that pragma_table_info reports.
			continue
		}
		col.Generated = hidden == 2 || hidden == 3

		if col.BaseType == "" {
			// The TABLE_INFO pragma doesn't return column types for virtual tables.
			//
			// REVISIT: This logic should be pulled out into a separate query for
			// all "untyped" columns, instead of invoking it for every untyped column.
			if col.BaseType, err = getTypeOfColumn(ctx, db, tblMeta.Name, col.Name); err != nil {
				return nil, err
			}
			progress.Incr(ctx, 1)
		}

		col.PrimaryKey = pkValue.Int64 > 0 // pkVal can be 0,1,2 etc
		col.ColumnType = col.BaseType
		col.Nullable = notnull == 0
		col.DefaultValue = defaultValue.String
		col.Kind = kindFromDBTypeName(ctx, col.Name, col.BaseType, nil)

		tblMeta.Columns = append(tblMeta.Columns, col)
	}

	err = rows.Err()
	if err != nil {
		return nil, errw(err)
	}

	// The CREATE DDL in sqlite_master drives the metadata that SQLite
	// exposes nowhere else: view definitions, CHECK constraints,
	// AUTOINCREMENT, and generated-column expressions.
	ddl, err := getTableDDL(ctx, db, tblMeta.Name)
	if err != nil {
		return nil, err
	}
	if tblMeta.TableType == sqlz.TableTypeView {
		tblMeta.ViewDefinition = ddl
	}

	// Triggers attach to both tables and views (INSTEAD OF triggers).
	tblMeta.Triggers, err = getTableTriggers(ctx, db, tblMeta.Name)
	if err != nil {
		return nil, err
	}

	// pragma_foreign_key_list / pragma_index_list are only meaningful
	// for real tables — views have no FKs or indexes, and SQLite
	// virtual tables (FTS5, r-tree, etc.) can error on these pragmas
	// depending on the module. Mirror the same guard the source-level
	// path uses in getAllTableMetadata.
	if tblMeta.TableType != sqlz.TableTypeTable {
		return tblMeta, nil
	}

	applyTableDDLMetadata(ctx, ddl, tblMeta)

	outgoing, err := getTableForeignKeys(ctx, db, tblMeta.Name)
	if err != nil {
		return nil, err
	}
	incoming, err := getTableIncomingFKs(ctx, db, tblMeta.Name)
	if err != nil {
		return nil, err
	}
	tblMeta.FK = metadata.NewFKGroup(outgoing, incoming)

	tblMeta.UniqueConstraints, tblMeta.Indexes, err = getTableIndexes(ctx, db, tblMeta.Name)
	if err != nil {
		return nil, err
	}

	return tblMeta, nil
}

// getTableIndexes returns the unique constraints and the physical
// indexes declared on tblName, using SQLite's pragma_index_list and
// pragma_index_info. Unique constraints are derived from index_list
// rows whose origin is 'u' (CREATE TABLE ... UNIQUE / ALTER TABLE ADD
// CONSTRAINT ... UNIQUE); PK-backing indexes have origin 'pk'; manual
// CREATE INDEX statements have origin 'c'.
//
// Returns (uniqueConstraints, indexes, error). Both slices may be nil
// for tables without any constraints/indexes.
func getTableIndexes(ctx context.Context, db sqlz.DB, tblName string) (
	[]*metadata.UniqueConstraint, []*metadata.Index, error,
) {
	log := lg.FromContext(ctx)

	// pragma_index_list returns: seq, name, unique, origin, partial.
	const qList = `SELECT name, "unique", origin
FROM pragma_index_list(?)
ORDER BY seq`

	rows, err := db.QueryContext(ctx, qList, tblName)
	if err != nil {
		return nil, nil, errw(err)
	}

	type indexInfo struct {
		name, origin string
		unique       bool
	}
	var infos []indexInfo
	for rows.Next() {
		var (
			name, origin string
			uniqueFlag   int64
		)
		if err = rows.Scan(&name, &uniqueFlag, &origin); err != nil {
			sqlz.CloseRows(log, rows)
			return nil, nil, errw(err)
		}
		infos = append(infos, indexInfo{name: name, origin: origin, unique: uniqueFlag != 0})
	}
	if err = rows.Err(); err != nil {
		sqlz.CloseRows(log, rows)
		return nil, nil, errw(err)
	}
	sqlz.CloseRows(log, rows)

	var (
		uniques []*metadata.UniqueConstraint
		indexes []*metadata.Index
	)
	for _, info := range infos {
		cols, err := getIndexColumns(ctx, db, info.name)
		if err != nil {
			return nil, nil, err
		}
		if metadata.AllExpressionKeys(cols) {
			log.Debug("sqlite3: dropping index with only expression keys",
				"table", tblName, "index", info.name)
			continue
		}

		idx := &metadata.Index{
			Name:    info.name,
			Table:   tblName,
			Columns: cols,
			Unique:  info.unique,
			Primary: info.origin == "pk",
		}
		indexes = append(indexes, idx)

		// origin='u' marks an index that backs a UNIQUE constraint
		// declared in CREATE TABLE / ALTER TABLE. The PK is reported
		// separately on Column.PrimaryKey, so we don't repeat it here.
		// SQLite prohibits expressions in UNIQUE constraints, so an
		// origin='u' index is always plain-column: cols never carries
		// the "" expression sentinel on this branch.
		if info.origin == "u" {
			uniques = append(uniques, &metadata.UniqueConstraint{
				Name:    info.name,
				Table:   tblName,
				Columns: cols,
			})
		}
	}
	return uniques, indexes, nil
}

// getIndexColumns returns the columns of an index in key order, using
// SQLite's pragma_index_info. Expression-based index entries (with a NULL
// column name) are recorded as the empty-string sentinel; see
// metadata.Index.Columns.
func getIndexColumns(ctx context.Context, db sqlz.DB, idxName string) ([]string, error) {
	log := lg.FromContext(ctx)
	const q = `SELECT name FROM pragma_index_info(?) ORDER BY seqno`
	rows, err := db.QueryContext(ctx, q, idxName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var cols []string
	for rows.Next() {
		var name sql.NullString
		if err = rows.Scan(&name); err != nil {
			return nil, errw(err)
		}
		// A NULL name marks an expression key position; record the
		// empty-string sentinel so Columns preserves the index's key
		// arity and the position of the expression. See
		// metadata.Index.Columns.
		cols = append(cols, name.String)
	}
	return cols, errw(rows.Err())
}

// getTableForeignKeys returns the outgoing foreign-key constraints
// declared on tblName, using SQLite's pragma_foreign_key_list. Returns
// nil if the table has no foreign keys. Composite foreign keys are
// returned as a single ForeignKey whose Columns/RefColumns slices are
// ordered by the pragma's seq field.
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
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

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

// getTableIncomingFKs returns the foreign-key constraints declared on
// other tables in this database whose referenced side is tblName. The
// query joins sqlite_master to pragma_foreign_key_list so the entire
// schema is swept in a single round-trip rather than via Go-side
// iteration. SQLite's pragma_foreign_key_list is per-referencing-table,
// so unlike the other drivers there's no native "what points at me?"
// view to filter on directly.
func getTableIncomingFKs(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)

	// Filter virtual tables out of the m.type='table' set: their
	// pragma_foreign_key_list may error on some VTAB modules and they
	// can't carry FK constraints anyway.
	const q = `SELECT m.name AS fk_table, fkl.id, fkl.seq, fkl."from", fkl."to", fkl.on_update, fkl.on_delete
FROM sqlite_master AS m
JOIN pragma_foreign_key_list(m.name) AS fkl
WHERE m.type = 'table'
  AND m.name NOT LIKE 'sqlite_%'
  AND substr(COALESCE(m.sql, ''), 1, 20) != 'CREATE VIRTUAL TABLE'
  AND fkl."table" = ?
ORDER BY m.name, fkl.id, fkl.seq`

	rows, err := db.QueryContext(ctx, q, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type fkKey struct {
		table string
		id    int64
	}
	byKey := map[fkKey]*metadata.ForeignKey{}
	var fks []*metadata.ForeignKey
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		var (
			fkTable            string
			id, seq            int64
			fromCol, toCol     string
			onUpdate, onDelete sql.NullString
		)
		if err = rows.Scan(&fkTable, &id, &seq, &fromCol, &toCol, &onUpdate, &onDelete); err != nil {
			return nil, errw(err)
		}

		k := fkKey{table: fkTable, id: id}
		fk, ok := byKey[k]
		if !ok {
			fk = &metadata.ForeignKey{
				Table:    fkTable,
				RefTable: tblName,
				OnDelete: onDelete.String,
				OnUpdate: onUpdate.String,
			}
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, fromCol)
		fk.RefColumns = append(fk.RefColumns, toCol)
	}
	return fks, errw(rows.Err())
}

// getAllTableMetadata gets metadata for each of the
// non-system tables in db's schema. Arg schemaName is used to
// set Table.FQName; it is not used to select which schema
// to introspect.
// The supplied incr func should be invoked for each row read from the DB.
func getAllTableMetadata(ctx context.Context, db sqlz.DB, schemaName string) ([]*metadata.Table, error) {
	log := lg.FromContext(ctx)
	// This query returns a row for each column of each table,
	// order by table name then col id (ordinal).
	// Results will look like:
	//
	// table_name	type	cid	name		type		"notnull"	dflt_value	pk
	// actor		table	0	actor_id	numeric		1			<null>		1
	// actor		table	1	first_name	VARCHAR(45)	1			<null>		0
	// actor		table	2	last_name	VARCHAR(45)	1			<null>		0
	// actor		table	3	last_update	TIMESTAMP	1			<null>		0
	// address		table	0	address_id	int			1			<null>		1
	// address		table	1	address		VARCHAR(50)	1			<null>		0
	// address		table	2	address2	VARCHAR(50)	0			NULL		0
	// address		table	3	district	VARCHAR(20)	1			<null>		0
	//
	// Note: dflt_value of col "address2" is the string "NULL", rather
	// that NULL value itself.
	// pragma_table_xinfo (rather than table_info) is joined so generated
	// columns are reported; p.hidden distinguishes them (2/3) and flags
	// virtual-table hidden columns (1) for exclusion below.
	// m.sql (the CREATE DDL) repeats for every column row in the join; it is
	// captured once per table during the scan via tblDDLs, eliminating a
	// per-table getTableDDL round-trip in the post-scan loop.
	const query = `
SELECT m.name as table_name, m.type, p.cid, p.name, p.type, p.'notnull' as 'notnull', p.dflt_value, p.pk, p.hidden,
(substr(m.sql, 0, 21) == 'CREATE VIRTUAL TABLE') AS is_virtual,
COALESCE(m.sql, '') AS table_ddl
FROM sqlite_master AS m JOIN pragma_table_xinfo(m.name) AS p
ORDER BY m.name, p.cid
`

	var (
		tblMetas        []*metadata.Table
		tblNames        []string
		tblDDLs         = make(map[string]string)
		curTblName      string
		curTblType      string
		curTblDDL       string
		curTblIsVirtual bool
		curTblMeta      *metadata.Table
	)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		col := &metadata.Column{}
		var notnull, hidden int64
		colDefault := &sql.NullString{}
		pkValue := &sql.NullInt64{}

		err = rows.Scan(
			&curTblName,
			&curTblType,
			&col.Position,
			&col.Name,
			&col.BaseType,
			&notnull,
			colDefault,
			pkValue,
			&hidden,
			&curTblIsVirtual,
			&curTblDDL,
		)
		if err != nil {
			return nil, errw(err)
		}

		if strings.HasPrefix(curTblName, "sqlite_") {
			// Skip system table "sqlite_sequence" etc.
			continue
		}

		if curTblMeta == nil || curTblMeta.Name != curTblName {
			// On our first time encountering a new table name, we create a new Table.
			// Capture the DDL from m.sql (same for all rows of this table).
			tblDDLs[curTblName] = curTblDDL
			curTblMeta = &metadata.Table{
				Name:        curTblName,
				FQName:      schemaName + "." + curTblName,
				Size:        nil, // No easy way of getting the storage size of a table
				DBTableType: curTblType,
			}

			switch {
			case curTblIsVirtual:
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

		if hidden == 1 {
			// Virtual-table hidden column: excluded to match the
			// user-visible column set pragma_table_info reports. The
			// owning table is still registered above.
			continue
		}
		col.Generated = hidden == 2 || hidden == 3

		if col.BaseType == "" {
			// The TABLE_INFO pragma doesn't return column types for virtual tables.
			//
			// REVISIT: This logic should be pulled out into a separate query for
			// all "untyped" columns, instead of invoking it for every untyped column.
			if col.BaseType, err = getTypeOfColumn(ctx, db, curTblName, col.Name); err != nil {
				return nil, err
			}
			progress.Incr(ctx, 1)
		}

		col.PrimaryKey = pkValue.Int64 > 0 // pkVal can be 0,1,2 etc
		col.ColumnType = col.BaseType
		col.Nullable = notnull == 0
		col.DefaultValue = colDefault.String
		col.Kind = kindFromDBTypeName(ctx, col.Name, col.BaseType, nil)

		curTblMeta.Columns = append(curTblMeta.Columns, col)
	}

	err = rows.Err()
	if err != nil {
		return nil, errw(err)
	}

	// Separately, we need to get the row counts for the tables
	var rowCounts []int64
	rowCounts, err = getTblRowCounts(ctx, db, tblNames)
	if err != nil {
		return nil, errw(err)
	}

	for i := range rowCounts {
		tblMetas[i].RowCount = rowCounts[i]
	}

	// Batch-fetch all triggers in a single round-trip (replaces N per-table
	// getTableTriggers calls).
	allTriggers, err := getAllTableTriggers(ctx, db)
	if err != nil {
		return nil, err
	}

	// Populate outgoing foreign keys, unique constraints, and indexes
	// per table. Cross-table linking (FK.Incoming) is handled at the
	// Source level by metadata.LinkForeignKeys.
	for _, tblMeta := range tblMetas {
		// The DDL was captured from m.sql during the bulk scan above,
		// so no additional round-trip is needed here.
		ddl := tblDDLs[tblMeta.Name]
		if tblMeta.TableType == sqlz.TableTypeView {
			tblMeta.ViewDefinition = ddl
		}

		// Triggers attach to both tables and views (INSTEAD OF triggers).
		tblMeta.Triggers = allTriggers[tblMeta.Name]

		// pragma_foreign_key_list / index_list are only meaningful for
		// tables, not views.
		if tblMeta.TableType != sqlz.TableTypeTable {
			continue
		}

		applyTableDDLMetadata(ctx, ddl, tblMeta)

		outgoing, err := getTableForeignKeys(ctx, db, tblMeta.Name)
		if err != nil {
			return nil, err
		}
		tblMeta.FK = metadata.NewFKGroup(outgoing, nil)

		tblMeta.UniqueConstraints, tblMeta.Indexes, err = getTableIndexes(ctx, db, tblMeta.Name)
		if err != nil {
			return nil, err
		}
	}

	return tblMetas, nil
}

// getTblRowCounts returns the number of rows in each table. If a table named
// in tblNames is dropped by concurrent DDL between enumeration and the COUNT
// batch here, the batch fails with "no such table"; getTblRowCounts then falls
// back to per-table COUNTs for that batch (countTblsIndividually) and records
// -1 for any table that has since vanished, so callers can detect (or skip).
func getTblRowCounts(ctx context.Context, db sqlz.DB, tblNames []string) ([]int64, error) {
	log := lg.FromContext(ctx)

	// See: https://stackoverflow.com/questions/7524612/how-to-count-rows-from-multiple-tables-in-sqlite
	//
	// Several approaches were benchmarked. Ultimately the union-based
	// query was selected.
	//
	// BenchmarkGetTblRowCounts/benchGetTblRowCountsBaseline-16         	     864	  43631750 ns/op
	// BenchmarkGetTblRowCounts/getTblRowCounts-16                      	    3948	   9126191 ns/op
	//
	// That query looks like:
	//
	//  SELECT COUNT(*) FROM "actor"
	//  UNION ALL
	//  SELECT COUNT(*) FROM "address"
	//  UNION ALL
	//  SELECT COUNT(*) FROM "category"
	//
	// Note that there is a limit (SQLITE_MAX_COMPOUND_SELECT)
	// to the number of "terms" (SELECT clauses) in a query.
	// See https://www.sqlite.org/limits.html#max_compound_select
	//
	// Thus if len(tblNames) > 500, we need to execute multiple queries.
	const maxCompoundSelect = 500

	var (
		tblCounts = make([]int64, len(tblNames))
		sb        strings.Builder
		query     string
		terms     int
		j         int
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

		query = sb.String()

		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			wrapped := errw(err)
			if errz.Has[*driver.NotExistError](wrapped) {
				// A table enumerated earlier in the scan was dropped by
				// concurrent DDL before we could COUNT it, which fails the
				// whole compound statement. Fall back to per-table COUNTs
				// across the current batch, recording -1 for any name that
				// has since vanished, so one dropped table doesn't abort the
				// whole source scan.
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
			err = rows.Scan(&tblCounts[j])
			if err != nil {
				sqlz.CloseRows(log, rows)
				return nil, errw(err)
			}
			j++
			progress.Incr(ctx, 1)
			debugz.DebugSleep(ctx)
		}

		if err = rows.Err(); err != nil {
			sqlz.CloseRows(log, rows)
			return nil, errw(err)
		}

		err = rows.Close()
		if err != nil {
			return nil, errw(err)
		}

		terms = 0
		sb.Reset()
	}

	return tblCounts, nil
}

// countTblsIndividually issues a per-table SELECT COUNT(*) for each name in
// names, writing the result to the matching slot in counts. Tables that have
// vanished (NotExistError) are recorded as -1; any other error aborts. This is
// the fallback path used by getTblRowCounts when the UNION ALL batch fails
// because of a concurrent DROP TABLE.
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
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
	}
	return nil
}

// getTableDDL returns the CREATE statement text recorded in
// sqlite_master for the named table or view, or empty string if no such
// row exists (or its sql is NULL, as for some auto-created objects).
func getTableDDL(ctx context.Context, db sqlz.DB, tblName string) (string, error) {
	const q = `SELECT COALESCE(sql, '') FROM sqlite_master
WHERE name = ? AND type IN ('table','view') LIMIT 1`
	var ddl string
	err := db.QueryRowContext(ctx, q, tblName).Scan(&ddl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", errw(err)
	}
	progress.Incr(ctx, 1)
	return ddl, nil
}

// applyTableDDLMetadata parses a table's CREATE DDL to populate the
// metadata SQLite exposes nowhere else: CHECK constraints, the
// AUTOINCREMENT column flag, and generated-column expressions. Parse
// failures are logged at debug level and swallowed; the column and
// constraint metadata already gathered via pragmas stays intact, and
// inspect never fails on un-parseable DDL.
func applyTableDDLMetadata(ctx context.Context, ddl string, tblMeta *metadata.Table) {
	if ddl == "" {
		return
	}
	log := lg.FromContext(ctx)

	if checks, err := sqlparser.ExtractCheckConstraints(ddl); err != nil {
		log.Debug("sqlite3: failed to parse CHECK constraints from DDL; skipping",
			lga.Table, tblMeta.Name, lga.Err, err)
	} else {
		for _, c := range checks {
			tblMeta.CheckConstraints = append(tblMeta.CheckConstraints, &metadata.CheckConstraint{
				Name:   c.Name,
				Table:  tblMeta.Name,
				Clause: c.Clause,
			})
		}
	}

	colInfo, err := sqlparser.ExtractColumnDDLInfo(ddl)
	if err != nil {
		log.Debug("sqlite3: failed to parse column DDL info; skipping",
			lga.Table, tblMeta.Name, lga.Err, err)
		return
	}
	for _, col := range tblMeta.Columns {
		info, ok := colInfo[col.Name]
		if !ok {
			continue
		}
		col.AutoIncrement = info.AutoIncrement
		// Only attach an expression to a column the pragma already
		// flagged as generated, so a stray AS-clause parse can't
		// mislabel an ordinary column.
		if col.Generated && info.GeneratedExpr != "" {
			col.GeneratedExpr = info.GeneratedExpr
		}
	}
}

// buildSQLiteTrigger constructs a *metadata.Trigger from its raw parts via
// the shared sqlparser.BuildTrigger, logging a parse failure with the
// sqlite3 driver prefix. The graceful fallback (raw Definition kept,
// structured fields left empty) and Trigger.Enabled staying nil live in
// sqlparser.BuildTrigger, shared with the rqlite driver.
func buildSQLiteTrigger(ctx context.Context, name, tblName, ddl string) *metadata.Trigger {
	trg, parseErr := sqlparser.BuildTrigger(name, tblName, ddl)
	if parseErr != nil {
		lg.FromContext(ctx).Debug("sqlite3: failed to parse trigger DDL; keeping raw definition",
			lga.Name, name, lga.Err, parseErr)
	}
	return trg
}

// getAllTableTriggers fetches all triggers for the database in a single
// round-trip and returns them grouped by table name. It is the bulk
// counterpart of getTableTriggers; per-trigger construction is shared
// via buildSQLiteTrigger.
func getAllTableTriggers(ctx context.Context, db sqlz.DB) (map[string][]*metadata.Trigger, error) {
	log := lg.FromContext(ctx)
	const q = `SELECT name, tbl_name, COALESCE(sql, '') FROM sqlite_master
WHERE type = 'trigger' ORDER BY tbl_name, name`
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	byTable := make(map[string][]*metadata.Trigger)
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
		var name, tblName, ddl string
		if err = rows.Scan(&name, &tblName, &ddl); err != nil {
			return nil, errw(err)
		}
		byTable[tblName] = append(byTable[tblName], buildSQLiteTrigger(ctx, name, tblName, ddl))
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return byTable, nil
}

// getTableTriggers returns the triggers attached to tblName, reading them
// from sqlite_master (type='trigger'). Each trigger's raw DDL is parsed
// for its timing (BEFORE/AFTER/INSTEAD OF) and firing events
// (INSERT/UPDATE/DELETE); a parse failure keeps the raw Definition and
// leaves the structured fields empty rather than failing inspect.
// Trigger.Enabled stays nil; SQLite has no enabled/disabled concept.
// This per-table function is used by the single-table path (getTableMetadata);
// the bulk path (getAllTableMetadata) uses getAllTableTriggers instead.
func getTableTriggers(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.Trigger, error) {
	log := lg.FromContext(ctx)
	const q = `SELECT name, COALESCE(sql, '') FROM sqlite_master
WHERE type = 'trigger' AND tbl_name = ? ORDER BY name`
	rows, err := db.QueryContext(ctx, q, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var triggers []*metadata.Trigger
	for rows.Next() {
		progress.Incr(ctx, 1)
		var name, ddl string
		if err = rows.Scan(&name, &ddl); err != nil {
			return nil, errw(err)
		}
		triggers = append(triggers, buildSQLiteTrigger(ctx, name, tblName, ddl))
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return triggers, nil
}

// getTypeOfColumn executes "SELECT typeof(colName)", returning the first result.
// Empty string is returned if there are no rows in that table, as SQLite determines
// type on a per-cell basis, not per-column.
func getTypeOfColumn(ctx context.Context, db sqlz.DB, tblName, colName string) (string, error) {
	colTypeQuery := fmt.Sprintf(`SELECT typeof(%s) FROM %s LIMIT 1`,
		stringz.DoubleQuote(colName), stringz.DoubleQuote(tblName))

	var colType string
	if err := db.QueryRowContext(ctx, colTypeQuery).Scan(&colType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}

		return "", errw(err)
	}

	return colType, nil
}

// semverRx matches a leading dotted-numeric version token (up to three parts).
var semverRx = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// parseSemver normalizes a SQLite version string to canonical semver (e.g.
// "v3.45.1").
func parseSemver(raw string) (string, error) {
	m := semverRx.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", errz.Errorf("no semver in sqlite version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid sqlite semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver.
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT sqlite_version()").Scan(&raw); err != nil {
		return "", errw(err)
	}
	return parseSemver(raw)
}
