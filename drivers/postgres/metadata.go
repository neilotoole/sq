package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/neilotoole/errgroup"
	"github.com/neilotoole/lg"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
)

// kindFromDBTypeName determines the kind.Kind from the database
// type name. For example, "VARCHAR" -> kind.Text.
// See https://www.postgresql.org/docs/9.5/datatype.html
func kindFromDBTypeName(log lg.Log, colName, dbTypeName string) kind.Kind {
	var knd kind.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	switch dbTypeName {
	default:
		log.Warnf("Unknown Postgres database type '%s' for column '%s': using %s", dbTypeName, colName, kind.Unknown)
		knd = kind.Unknown
	case "":
		knd = kind.Unknown
	case "INT", "INTEGER", "INT2", "INT4", "INT8", "SMALLINT", "BIGINT":
		knd = kind.Int
	case "CHAR", "CHARACTER", "VARCHAR", "TEXT", "BPCHAR", "CHARACTER VARYING":
		knd = kind.Text
	case "BYTEA":
		knd = kind.Bytes
	case "BOOL", "BOOLEAN":
		knd = kind.Bool
	case "TIMESTAMP", "TIMESTAMPTZ", "TIMESTAMP WITHOUT TIME ZONE":
		knd = kind.Datetime
	case "TIME", "TIMETZ", "TIME WITHOUT TIME ZONE":
		knd = kind.Time
	case "DATE":
		knd = kind.Date
	case "INTERVAL": // interval meaning time duration
		knd = kind.Text
	case "FLOAT", "FLOAT4", "FLOAT8", "DOUBLE", "DOUBLE PRECISION":
		knd = kind.Float
	case "UUID":
		knd = kind.Text
	case "DECIMAL", "NUMERIC", "MONEY":
		knd = kind.Decimal
	case "JSON", "JSONB":
		knd = kind.Text
	case "BIT", "VARBIT":
		knd = kind.Text
	case "XML":
		knd = kind.Text
	case "BOX", "CIRCLE", "LINE", "LSEG", "PATH", "POINT", "POLYGON":
		knd = kind.Text
	case "CIDR", "INET", "MACADDR":
		knd = kind.Text
	case "USER-DEFINED":
		// REVISIT: How to handle USER-DEFINED type?
		knd = kind.Text
	case "TSVECTOR":
		// REVISIT: how to handle TSVECTOR type?
		knd = kind.Text
	case "ARRAY":
		// REVISIT: how to handle ARRAY type?
		knd = kind.Text
	}

	return knd
}

// setScanType ensures that ctd's scan type field is set appropriately.
func setScanType(log lg.Log, ctd *sqlz.ColumnTypeData, knd kind.Kind) {
	if knd == kind.Decimal {
		// Force the use of string for decimal, as the driver will
		// sometimes prefer float.
		ctd.ScanType = sqlz.RTypeNullString
		return
	}

	// Need to switch to the nullable scan types because the
	// backing driver doesn't report nullable info accurately.
	ctd.ScanType = toNullableScanType(log, ctd.Name, ctd.DatabaseTypeName, knd, ctd.ScanType)
}

// toNullableScanType returns the nullable equivalent of the scan type
// reported by the postgres driver's ColumnType.ScanType. This is necessary
// because the pgx driver does not support the stdlib sql
// driver.RowsColumnTypeNullable interface.
func toNullableScanType(log lg.Log, colName, dbTypeName string, knd kind.Kind, pgScanType reflect.Type) reflect.Type {
	var nullableScanType reflect.Type

	switch pgScanType {
	default:
		// If we don't recognize the scan type (likely it's interface{}),
		// we explicitly switch through the db type names that we know.
		// At this time, we will use NullString for all unrecognized
		// scan types, but nonetheless we switch through the known db type
		// names so that we see the log warning for truly unknown types.
		switch dbTypeName {
		default:
			log.Warnf("Unknown postgres scan type: col(%s) --> scan(%s) --> db(%s) --> kind(%s): using %s",
				colName, pgScanType, dbTypeName, knd, sqlz.RTypeNullString)
			nullableScanType = sqlz.RTypeNullString
		case "":
			// NOTE: the pgx driver currently reports an empty dbTypeName for certain
			//  cols such as XML or MONEY.
			nullableScanType = sqlz.RTypeNullString
		case "TIME":
			nullableScanType = sqlz.RTypeNullString
		case "BIT", "VARBIT":
			nullableScanType = sqlz.RTypeNullString
		case "BPCHAR":
			nullableScanType = sqlz.RTypeNullString
		case "BOX", "CIRCLE", "LINE", "LSEG", "PATH", "POINT", "POLYGON":
			nullableScanType = sqlz.RTypeNullString
		case "CIDR", "INET", "MACADDR":
			nullableScanType = sqlz.RTypeNullString
		case "INTERVAL":
			nullableScanType = sqlz.RTypeNullString
		case "JSON", "JSONB":
			nullableScanType = sqlz.RTypeNullString
		case "XML":
			nullableScanType = sqlz.RTypeNullString
		case "UUID":
			nullableScanType = sqlz.RTypeNullString
		}

	case sqlz.RTypeInt64, sqlz.RTypeInt, sqlz.RTypeInt8, sqlz.RTypeInt16, sqlz.RTypeInt32, sqlz.RTypeNullInt64:
		nullableScanType = sqlz.RTypeNullInt64

	case sqlz.RTypeFloat32, sqlz.RTypeFloat64, sqlz.RTypeNullFloat64:
		nullableScanType = sqlz.RTypeNullFloat64

	case sqlz.RTypeString, sqlz.RTypeNullString:
		nullableScanType = sqlz.RTypeNullString

	case sqlz.RTypeBool, sqlz.RTypeNullBool:
		nullableScanType = sqlz.RTypeNullBool

	case sqlz.RTypeTime, sqlz.RTypeNullTime:
		nullableScanType = sqlz.RTypeNullTime

	case sqlz.RTypeBytes:
		nullableScanType = sqlz.RTypeBytes
	}

	return nullableScanType
}

func getSourceMetadata(ctx context.Context, log lg.Log, src *source.Source, db sqlz.DB) (*source.Metadata, error) {
	md := &source.Metadata{
		Handle:       src.Handle,
		Location:     src.Location,
		SourceType:   src.Type,
		DBDriverType: src.Type,
	}

	var schema string
	const summaryQuery = `SELECT current_catalog, current_schema(), pg_database_size(current_catalog),
current_setting('server_version'), version(), "current_user"()`

	err := db.QueryRowContext(ctx, summaryQuery).
		Scan(&md.Name, &schema, &md.Size, &md.DBVersion, &md.DBProduct, &md.User)
	if err != nil {
		return nil, errz.Err(err)
	}

	md.FQName = md.Name + "." + schema

	md.DBVars, err = getPgSettings(ctx, log, db)
	if err != nil {
		return nil, err
	}

	tblNames, err := getAllTableNames(ctx, log, db)
	if err != nil {
		return nil, err
	}

	g, gctx := errgroup.WithContextN(ctx, driver.Tuning.ErrgroupNumG, driver.Tuning.ErrgroupQSize)
	tblMetas := make([]*source.TableMetadata, len(tblNames))

	for i := range tblNames {
		select {
		case <-gctx.Done():
			return nil, errz.Err(gctx.Err())
		default:
		}

		i := i
		g.Go(func() error {
			tblMeta, mdErr := getTableMetadata(gctx, log, db, tblNames[i])
			if mdErr != nil {
				if hasErrCode(mdErr, errCodeRelationNotExist) {
					// If the table is dropped while we're collecting metadata,
					// for example, we log a warning and suppress the error.
					log.Warnf("table metadata collection: table %q appears not to exist (continuing regardless): %v",
						tblNames[i], mdErr)
					return nil
				}
				return mdErr
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

// hasErrCode returns true if err (or its cause error)
// is of type *pgconn.PgError and err.Number equals code.
func hasErrCode(err error, code string) bool {
	err = errz.Cause(err)
	if err2, ok := err.(*pgconn.PgError); ok {
		return err2.Code == code
	}
	return false
}

const errCodeRelationNotExist = "42P01"

func getPgSettings(ctx context.Context, log lg.Log, db sqlz.DB) ([]source.DBVar, error) {
	rows, err := db.QueryContext(ctx, "SELECT name, setting FROM pg_settings ORDER BY name")
	if err != nil {
		return nil, errz.Err(err)
	}

	defer log.WarnIfCloseError(rows)
	var dbVars []source.DBVar

	for rows.Next() {
		v := source.DBVar{}
		err = rows.Scan(&v.Name, &v.Value)
		if err != nil {
			return nil, errz.Err(err)
		}
		dbVars = append(dbVars, v)
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return dbVars, nil
}

// getAllTable names returns all table (or view) names in the current
// catalog & schema.
func getAllTableNames(ctx context.Context, log lg.Log, db sqlz.DB) ([]string, error) {
	const tblNamesQuery = `SELECT table_name FROM information_schema.tables
WHERE table_catalog = current_catalog AND table_schema = current_schema()
ORDER BY table_name`

	rows, err := db.QueryContext(ctx, tblNamesQuery)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(rows)

	var tblNames []string
	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, errz.Err(err)
		}
		tblNames = append(tblNames, s)
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return tblNames, nil
}

func getTableMetadata(ctx context.Context, log lg.Log, db sqlz.DB, tblName string) (*source.TableMetadata, error) {
	const tblsQueryTpl = `SELECT table_catalog, table_schema, table_name, table_type, is_insertable_into,
  (SELECT COUNT(*) FROM "%s") AS table_row_count,
  pg_total_relation_size('%q') AS table_size,
  (SELECT '%q'::regclass::oid AS table_oid),
  obj_description('%q'::REGCLASS, 'pg_class') AS table_comment
FROM information_schema.tables
WHERE table_catalog = current_database()
AND table_schema = current_schema()
AND table_name = $1`
	tablesQuery := fmt.Sprintf(tblsQueryTpl, tblName, tblName, tblName, tblName)

	pgTbl := &pgTable{}
	err := db.QueryRowContext(ctx, tablesQuery, tblName).
		Scan(&pgTbl.tableCatalog, &pgTbl.tableSchema, &pgTbl.tableName, &pgTbl.tableType, &pgTbl.isInsertable, &pgTbl.rowCount, &pgTbl.size, &pgTbl.oid, &pgTbl.comment)
	if err != nil {
		return nil, errz.Err(err)
	}

	tblMeta := tblMetaFromPgTable(pgTbl)
	if tblMeta.Name != tblName {
		// Shouldn't happen, but we'll error if it does
		return nil, errz.Errorf("table %q not found in %s.%s", tblName, pgTbl.tableCatalog, pgTbl.tableSchema)
	}

	pgCols, err := getPgColumns(ctx, log, db, tblName)
	if err != nil {
		return nil, err
	}

	for _, pgCol := range pgCols {
		colMeta := colMetaFromPgColumn(log, pgCol)
		tblMeta.Columns = append(tblMeta.Columns, colMeta)
	}

	// We need to fetch the constraints to set the PK etc.
	pgConstraints, err := getPgConstraints(ctx, log, db, tblName)
	if err != nil {
		return nil, err
	}

	setTblMetaConstraints(log, tblMeta, pgConstraints)

	return tblMeta, nil
}

// pgTable holds query results for table metadata.
type pgTable struct {
	tableCatalog string
	tableSchema  string
	tableName    string
	tableType    string
	isInsertable sqlz.NullBool // Use driver.NullBool because "YES", "NO" values
	rowCount     int64
	size         sql.NullInt64
	oid          string
	comment      sql.NullString
}

func tblMetaFromPgTable(pgt *pgTable) *source.TableMetadata {
	md := &source.TableMetadata{
		Name:        pgt.tableName,
		FQName:      fmt.Sprintf("%s.%s.%s", pgt.tableCatalog, pgt.tableSchema, pgt.tableName),
		DBTableType: pgt.tableType,
		RowCount:    pgt.rowCount,
		Comment:     pgt.comment.String,
		Columns:     nil, // Note: columns are set independently later
	}

	if pgt.size.Valid && pgt.size.Int64 > 0 {
		md.Size = &pgt.size.Int64
	}

	switch md.DBTableType {
	case "BASE TABLE":
		md.TableType = sqlz.TableTypeTable
	case "VIEW":
		md.TableType = sqlz.TableTypeView
	default:
	}

	return md
}

// pgColumn holds query results for column metadata.
// See https://www.postgresql.org/docs/8.0/infoschema-columns.html
type pgColumn struct {
	tableCatalog           string
	tableSchema            string
	tableName              string
	columnName             string
	ordinalPosition        int64
	columnDefault          sql.NullString
	isNullable             sqlz.NullBool
	dataType               string
	characterMaximumLength sql.NullInt64
	characterOctetLength   sql.NullInt64
	numericPrecision       sql.NullInt64
	numericPrecisionRadix  sql.NullInt64
	numericScale           sql.NullInt64
	datetimePrecision      sql.NullInt64
	domainCatalog          sql.NullString
	domainSchema           sql.NullString
	domainName             sql.NullString
	udtCatalog             string
	udtSchema              string
	udtName                string
	isIdentity             sqlz.NullBool
	isGenerated            sql.NullString
	isUpdatable            sqlz.NullBool

	// comment holds any column comment. Note that this field is
	// not part of the standard postgres infoschema, but is
	// separately fetched.
	comment sql.NullString
}

// getPgColumns queries the column metadata for tblName.
func getPgColumns(ctx context.Context, log lg.Log, db sqlz.DB, tblName string) ([]*pgColumn, error) {
	// colsQuery gets column information from information_schema.columns.
	//
	// It also has a subquery to get column comments. See:
	//   - https://stackoverflow.com/a/22547588
	//   - https://dba.stackexchange.com/a/160668
	const colsQuery = `SELECT table_catalog,
  table_schema,
  table_name,
  column_name, 
  ordinal_position, 
  column_default,
  is_nullable,
  data_type,
  character_maximum_length,
  character_octet_length,
  numeric_precision,
  numeric_precision_radix,
  numeric_scale,
  datetime_precision,
  domain_catalog,
  domain_schema,
  domain_name,
  udt_catalog,
  udt_schema,
  udt_name,
  is_identity,
  is_generated,
  is_updatable,
  (
	SELECT
		pg_catalog.col_description(c.oid, cols.ordinal_position::int)
	FROM
		pg_catalog.pg_class c
	WHERE
		c.oid = (SELECT ('"' || cols.table_name || '"')::regclass::oid)
		AND c.relname = cols.table_name
	) AS column_comment
FROM information_schema.columns cols
WHERE cols.table_catalog = current_catalog AND cols.table_schema = current_schema() AND cols.table_name = $1
ORDER BY cols.table_catalog, cols.table_schema, cols.table_name, cols.ordinal_position`

	rows, err := db.QueryContext(ctx, colsQuery, tblName)
	if err != nil {
		return nil, errz.Err(err)
	}

	defer log.WarnIfCloseError(rows)

	var cols []*pgColumn
	for rows.Next() {
		col := &pgColumn{}
		err = scanPgColumn(rows, col)
		if err != nil {
			return nil, err
		}

		cols = append(cols, col)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return cols, nil
}

func scanPgColumn(rows *sql.Rows, c *pgColumn) error {
	err := rows.Scan(&c.tableCatalog, &c.tableSchema, &c.tableName, &c.columnName, &c.ordinalPosition,
		&c.columnDefault, &c.isNullable, &c.dataType, &c.characterMaximumLength, &c.characterOctetLength,
		&c.numericPrecision, &c.numericPrecisionRadix, &c.numericScale,
		&c.datetimePrecision, &c.domainCatalog, &c.domainSchema, &c.domainName,
		&c.udtCatalog, &c.udtSchema, &c.udtName,
		&c.isIdentity, &c.isGenerated, &c.isUpdatable, &c.comment)
	return errz.Err(err)
}

func colMetaFromPgColumn(log lg.Log, pgCol *pgColumn) *source.ColMetadata {
	colMeta := &source.ColMetadata{
		Name:         pgCol.columnName,
		Position:     pgCol.ordinalPosition,
		PrimaryKey:   false, // Note that PrimaryKey is set separately from pgConstraint.
		BaseType:     pgCol.udtName,
		ColumnType:   pgCol.dataType,
		Kind:         kindFromDBTypeName(log, pgCol.columnName, pgCol.udtName),
		Nullable:     pgCol.isNullable.Bool,
		DefaultValue: pgCol.columnDefault.String,
		Comment:      pgCol.comment.String,
	}
	return colMeta
}

// getPgConstraints returns a slice of pgConstraint. If tblName is
// empty, constraints for all tables in the current catalog & schema
// are returned. If tblName is specified, constraints just for that
// table are returned.
func getPgConstraints(ctx context.Context, log lg.Log, db sqlz.DB, tblName string) ([]*pgConstraint, error) {
	var args []interface{}
	query := `SELECT kcu.table_catalog,kcu.table_schema,kcu.table_name,kcu.column_name,
    kcu.ordinal_position,tc.constraint_name,tc.constraint_type,
    (
       SELECT pg_catalog.pg_get_constraintdef(pgc.oid, true)
       FROM pg_catalog.pg_constraint pgc
       WHERE pgc.conrelid = (SELECT ('"' || kcu.table_name || '"')::regclass::oid)
       AND pgc.conname = tc.constraint_name
       LIMIT 1
    )  AS constraint_def,
    (
       SELECT pgc.confrelid::regclass
       FROM pg_catalog.pg_constraint pgc
       WHERE pgc.conrelid = (SELECT ('"' || kcu.table_name || '"')::regclass::oid)
       AND pgc.conname = tc.constraint_name
       AND pgc.confrelid > 0
       LIMIT 1
    )  AS constraint_fkey_table_name
FROM information_schema.key_column_usage AS kcu
    LEFT JOIN information_schema.table_constraints AS tc
    ON tc.constraint_name = kcu.constraint_name
WHERE kcu.table_catalog = current_catalog AND kcu.table_schema = current_schema()
`

	if tblName != "" {
		query += ` AND kcu.table_name = $1 `
		args = append(args, tblName)
	}

	query += ` ORDER BY kcu.table_catalog, kcu.table_schema, kcu.table_name, tc.constraint_type DESC, kcu.ordinal_position`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errz.Err(err)
	}
	defer log.WarnIfCloseError(rows)

	var constraints []*pgConstraint

	for rows.Next() {
		pgc := &pgConstraint{}
		err = rows.Scan(&pgc.tableCatalog, &pgc.tableSchema, &pgc.tableName, &pgc.columnName, &pgc.ordinalPosition,
			&pgc.constraintName, &pgc.constraintType, &pgc.constraintDef, &pgc.constraintFKeyTableName)
		if err != nil {
			return nil, errz.Err(err)
		}

		constraints = append(constraints, pgc)
	}
	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return constraints, nil
}

// pgConstraint holds query results for constraint metadata.
// This type is column-focused: that is, an instance is produced
// for each constraint/column pair. Thus, if a table has a
// composite primary key (col_a, col_b), two pgConstraint instances
// are produced.
type pgConstraint struct {
	tableCatalog    string
	tableSchema     string
	tableName       string
	columnName      string
	ordinalPosition int64
	constraintName  string
	constraintType  string
	constraintDef   string

	// constraintFKeyTableName holds the name of the table to which
	// a foreign-key constraint points to. This is null if this
	// constraint is not a foreign key.
	constraintFKeyTableName sql.NullString
}

// setTblMetaConstraints updates tblMeta with constraints found
// in pgConstraints.
func setTblMetaConstraints(log lg.Log, tblMeta *source.TableMetadata, pgConstraints []*pgConstraint) {
	for _, pgc := range pgConstraints {
		fqTblName := pgc.tableCatalog + "." + pgc.tableSchema + "." + pgc.tableName
		if fqTblName != tblMeta.FQName {
			continue
		}

		if pgc.constraintType == constraintTypePK {
			colMeta := tblMeta.Column(pgc.columnName)
			if colMeta == nil {
				// Shouldn't happen
				log.Warnf("No column %s.%s found matching constraint %q", tblMeta.Name, pgc.columnName, pgc.constraintName)
				continue
			}
			colMeta.PrimaryKey = true
		}
	}
}

const (
	constraintTypePK = "PRIMARY KEY"
	constraintTypeFK = "FOREIGN KEY"
)

// closeRows invokes rows.Err and rows.Close, returning
// an error if either of those methods returned an error.
func closeRows(rows *sql.Rows) error {
	if rows == nil {
		return nil
	}
	err1 := rows.Err()
	err2 := rows.Close()
	if err1 != nil {
		return errz.Err(err1)
	}
	return errz.Err(err2)
}
