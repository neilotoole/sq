package databricks

import (
	"context"
	"database/sql"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tablefq"
	"github.com/neilotoole/sq/libsq/driver"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// rowCountConcurrency limits the number of concurrent SELECT COUNT(*)
// queries issued while building table metadata.
const rowCountConcurrency = 8

// kindFromDBTypeName maps a Databricks type name to a sq kind.Kind. The name
// can come from two places: result-set column metadata (as reported by
// databricks-sql-go, e.g. "BIGINT", "TIMESTAMP_NTZ"), or information_schema
// (e.g. "LONG", "STRUCT", or "decimal(10,2)" for full_data_type). Matching
// is case-insensitive, and any parameters are ignored: "DECIMAL(10,2)",
// "array<int>" and "VARCHAR(20)" match as "DECIMAL", "ARRAY" and "VARCHAR".
//
// Type mapping:
//
//	Databricks type                                        sq kind
//	-----------------------------------------------------------------------
//	BOOLEAN                                                kind.Bool
//	TINYINT, SMALLINT, INT, BIGINT (BYTE, SHORT, LONG)     kind.Int
//	FLOAT, DOUBLE                                          kind.Float
//	DECIMAL                                                kind.Decimal
//	STRING, VARCHAR, CHAR                                  kind.Text
//	BINARY                                                 kind.Bytes
//	DATE                                                   kind.Date
//	TIMESTAMP, TIMESTAMP_NTZ                               kind.Datetime
//	ARRAY, MAP, STRUCT, VARIANT, INTERVAL, GEOGRAPHY,
//	GEOMETRY, UNION, USER_DEFINED                          kind.Text
//	VOID (NULL)                                            kind.Null
//
// ARRAY, MAP, STRUCT and VARIANT values are returned by databricks-sql-go
// as JSON strings, INTERVAL values as strings such as "1-2" or
// "1 02:03:04.000000000", and GEOGRAPHY and GEOMETRY values as WKT strings.
// They map to kind.Text, so the structure is flattened to text. This is the
// same trade-off made for ClickHouse arrays (see issue #545).
//
// Any other type name returns kind.Unknown, and a warning is logged.
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
	base := strings.ToUpper(strings.TrimSpace(dbTypeName))
	if i := strings.IndexAny(base, "(< "); i >= 0 {
		base = base[:i]
	}

	switch base {
	case "BOOLEAN", "BOOL":
		return kind.Bool
	case "TINYINT", "BYTE", "SMALLINT", "SHORT", "INT", "INTEGER", "BIGINT", "LONG":
		return kind.Int
	case "FLOAT", "REAL", "DOUBLE":
		return kind.Float
	case "DECIMAL", "DEC", "NUMERIC":
		return kind.Decimal
	case "STRING", "VARCHAR", "CHAR", "TEXT":
		return kind.Text
	case "BINARY":
		return kind.Bytes
	case "DATE":
		return kind.Date
	case "TIMESTAMP", "TIMESTAMP_NTZ", "TIMESTAMP_LTZ":
		return kind.Datetime
	case "ARRAY", "MAP", "STRUCT", "UNION", "VARIANT", "OBJECT", "USER_DEFINED",
		"INTERVAL", "INTERVAL_YEAR_MONTH", "INTERVAL_DAY_TIME",
		"GEOGRAPHY", "GEOMETRY":
		return kind.Text
	case "VOID", "NULL":
		return kind.Null
	default:
		if log != nil {
			log.Warn("Unknown Databricks column type: using kind.Unknown",
				lga.Type, dbTypeName, lga.Col, colName)
		}
		return kind.Unknown
	}
}

// setScanType sets colTypeData.ScanType, the type that sq scans the column
// value into, based on knd.
//
// databricks-sql-go does not report column nullability: sql.ColumnType
// Nullable returns ok=false. A Databricks column can hold NULL regardless,
// and the result of an expression such as a LEFT JOIN is nullable even when
// the source column is not. So every column is scanned into a nullable type.
//
//	Kind                  Scan type
//	--------------------------------------
//	Text, Unknown, Null   sql.NullString
//	Int                   sql.NullInt64
//	Float                 sql.NullFloat64
//	Decimal               decimal.NullDecimal
//	Bool                  sql.NullBool
//	Datetime, Date, Time  sql.NullTime
//	Bytes                 []byte
func setScanType(colTypeData *record.ColumnTypeData, knd kind.Kind) {
	switch knd { //nolint:exhaustive // other kinds fall through to the default
	case kind.Int:
		colTypeData.ScanType = sqlz.RTypeNullInt64
	case kind.Float:
		colTypeData.ScanType = sqlz.RTypeNullFloat64
	case kind.Decimal:
		colTypeData.ScanType = sqlz.RTypeNullDecimal
	case kind.Bool:
		colTypeData.ScanType = sqlz.RTypeNullBool
	case kind.Datetime, kind.Date, kind.Time:
		colTypeData.ScanType = sqlz.RTypeNullTime
	case kind.Bytes:
		colTypeData.ScanType = sqlz.RTypeBytes
	default:
		colTypeData.ScanType = sqlz.RTypeNullString
	}
}

// recordMetaFromColumnTypes builds the record.Meta for colTypes.
func recordMetaFromColumnTypes(ctx context.Context, colTypes []*sql.ColumnType) (record.Meta, error) {
	log := lg.FromContext(ctx)

	sColTypeData := make([]*record.ColumnTypeData, len(colTypes))
	ogColNames := make([]string, len(colTypes))
	for i, colType := range colTypes {
		knd := kindFromDBTypeName(log, colType.Name(), colType.DatabaseTypeName())
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

// getNewRecordFunc returns a driver.NewRecordFunc that converts a row
// scanned per rowMeta into a record.
func getNewRecordFunc(rowMeta record.Meta) driver.NewRecordFunc {
	return func(row []any) (record.Record, error) {
		rec, skipped := driver.NewRecordFromScanRow(rowMeta, row, nil)
		if len(skipped) > 0 {
			return nil, errz.Errorf("expected zero skipped cols but have %d: %v", len(skipped), skipped)
		}
		return rec, nil
	}
}

// semverRx matches a leading dotted-numeric version token, up to three parts.
var semverRx = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// parseSemver normalizes the output of the Spark version() function, e.g.
// "3.5.0 a1b2c3d4", to canonical semver, e.g. "v3.5.0". That is the Spark
// version of the engine behind the warehouse, not a Databricks release.
func parseSemver(raw string) (string, error) {
	m := semverRx.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", errz.Errorf("no semver in databricks version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid databricks semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver.
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&raw); err != nil {
		return "", errw(err)
	}
	return parseSemver(raw)
}

// DBProperties implements driver.SQLDriver.
func (d *driveri) DBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	return getDBProperties(ctx, db)
}

// getDBProperties returns the session properties surfaced by
// SourceMetadata.DBProperties.
func getDBProperties(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	var version, catalog, schma string
	err := db.QueryRowContext(ctx,
		"SELECT version(), current_catalog(), current_schema()").Scan(&version, &catalog, &schma)
	if err != nil {
		return nil, errw(err)
	}

	return map[string]any{
		"version": version,
		"catalog": catalog,
		"schema":  schma,
	}, nil
}

// infoSchema returns the quoted, catalog-qualified information_schema
// name for catalog, e.g. "`main`.information_schema". Unity Catalog exposes
// an information_schema in each catalog. The legacy hive_metastore catalog
// does not.
func infoSchema(catalog string) string {
	return stringz.BacktickQuote(catalog) + ".information_schema"
}

// getSourceMetadata returns metadata for the source. The source's schema and
// catalog are the session's current schema and catalog: the "catalog" and
// "schema" params of the location if given, else the workspace defaults.
func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB, noSchema bool) (*metadata.Source, error) {
	md := &metadata.Source{
		Handle:   src.Handle,
		Location: src.Location,
		Driver:   drivertype.Databricks,
		DBDriver: drivertype.Databricks,
	}

	var version, catalog, schma, user string
	err := db.QueryRowContext(ctx,
		"SELECT version(), current_catalog(), current_schema(), current_user()").
		Scan(&version, &catalog, &schma, &user)
	if err != nil {
		return nil, errw(err)
	}

	md.DBVersion = version
	if v, semverErr := parseSemver(version); semverErr != nil {
		lg.FromContext(ctx).Warn("Cannot derive db_semver from db_version",
			lga.Err, semverErr, lga.Version, version)
	} else {
		md.DBSemver = v
	}
	md.DBProduct = "Databricks " + version
	md.Catalog = catalog
	md.Schema = schma
	md.Name = catalog
	md.FQName = catalog + "." + schma
	md.User = user
	md.DBProperties = map[string]any{
		"version": version,
		"catalog": catalog,
		"schema":  schma,
	}

	if noSchema {
		return md, nil
	}

	if md.Tables, err = getTablesMetadata(ctx, db, catalog, schma, ""); err != nil {
		return nil, err
	}
	md.RecomputeTableCounts()

	return md, nil
}

// tableRow holds a row from information_schema.tables.
type tableRow struct {
	name    string
	dbType  string
	comment sql.NullString
}

// getTablesMetadata returns metadata for the tables and views of schma in
// catalog, drawn from catalog's information_schema.
//
// If only is not empty, just the table or view with that name is returned,
// which avoids a row count for every other table in the schema.
//
// Primary keys are read from information_schema as well, but note that in
// Databricks they are informational: they are not enforced. A failure to
// read primary keys or view definitions is logged and otherwise ignored,
// because the columns and tables are still useful without them.
func getTablesMetadata(ctx context.Context, db sqlz.DB, catalog, schma, only string) ([]*metadata.Table, error) {
	log := lg.FromContext(ctx)
	info := infoSchema(catalog)

	tblRows, err := queryTableRows(ctx, db, info, schma)
	if err != nil {
		return nil, err
	}
	if only != "" {
		tblRows = slices.DeleteFunc(tblRows, func(tr tableRow) bool { return tr.name != only })
	}
	if len(tblRows) == 0 {
		return []*metadata.Table{}, nil
	}

	cols, err := queryColumns(ctx, db, info, schma)
	if err != nil {
		return nil, err
	}

	pks, err := queryPrimaryKeys(ctx, db, info, schma)
	if err != nil {
		log.Warn("Databricks: failed to read primary keys; continuing without them",
			lga.Catalog, catalog, lga.Schema, schma, lga.Err, err)
	}

	viewDefs, err := queryViewDefinitions(ctx, db, info, schma)
	if err != nil {
		log.Warn("Databricks: failed to read view definitions; continuing without them",
			lga.Catalog, catalog, lga.Schema, schma, lga.Err, err)
	}

	tbls := make([]*metadata.Table, len(tblRows))
	for i, tr := range tblRows {
		tbl := &metadata.Table{
			Name:        tr.name,
			FQName:      catalog + "." + schma + "." + tr.name,
			DBTableType: tr.dbType,
			TableType:   tableTypeFromDBType(tr.dbType),
			Comment:     tr.comment.String,
			Columns:     cols[tr.name],
		}
		if tbl.Columns == nil {
			tbl.Columns = []*metadata.Column{}
		}
		for _, col := range tbl.Columns {
			if pks[tr.name][col.Name] {
				col.PrimaryKey = true
			}
		}
		if tbl.TableType != sqlz.TableTypeTable {
			tbl.ViewDefinition = viewDefs[tr.name]
		}
		tbls[i] = tbl
	}

	if err = fillRowCounts(ctx, db, catalog, schma, tbls); err != nil {
		return nil, err
	}

	return tbls, nil
}

// getTableMetadata returns metadata for a single table or view. If schma or
// catalog is empty, the session's current value is used.
func getTableMetadata(ctx context.Context, db sqlz.DB, catalog, schma, tblName string) (*metadata.Table, error) {
	if catalog == "" || schma == "" {
		var curCatalog, curSchema string
		err := db.QueryRowContext(ctx, "SELECT current_catalog(), current_schema()").
			Scan(&curCatalog, &curSchema)
		if err != nil {
			return nil, errw(err)
		}
		if catalog == "" {
			catalog = curCatalog
		}
		if schma == "" {
			schma = curSchema
		}
	}

	tbls, err := getTablesMetadata(ctx, db, catalog, schma, tblName)
	if err != nil {
		return nil, err
	}

	for _, tbl := range tbls {
		if tbl.Name == tblName {
			return tbl, nil
		}
	}

	return nil, driver.NewNotExistError(errz.Errorf("table %s.%s.%s not found", catalog, schma, tblName))
}

// queryTableRows returns the tables and views of schma, ordered by name.
func queryTableRows(ctx context.Context, db sqlz.DB, info, schma string) ([]tableRow, error) {
	q := "SELECT table_name, table_type, comment FROM " + info + ".tables " +
		"WHERE table_schema = ? ORDER BY table_name"
	rows, err := db.QueryContext(ctx, q, schma)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	var tblRows []tableRow
	for rows.Next() {
		var tr tableRow
		var dbType sql.NullString
		if err = rows.Scan(&tr.name, &dbType, &tr.comment); err != nil {
			return nil, errw(err)
		}
		tr.dbType = dbType.String
		tblRows = append(tblRows, tr)
	}
	return tblRows, errw(rows.Err())
}

// queryColumns returns the columns of the tables in schma, keyed by table name.
func queryColumns(ctx context.Context, db sqlz.DB, info, schma string) (map[string][]*metadata.Column, error) {
	q := "SELECT table_name, column_name, ordinal_position, is_nullable, data_type, " +
		"full_data_type, column_default, comment FROM " + info + ".columns " +
		"WHERE table_schema = ? ORDER BY table_name, ordinal_position"
	rows, err := db.QueryContext(ctx, q, schma)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	log := lg.FromContext(ctx)
	cols := map[string][]*metadata.Column{}
	for rows.Next() {
		var tblName, colName string
		var position sql.NullInt64
		var nullable, dataType, fullType, dflt, comment sql.NullString
		if err = rows.Scan(&tblName, &colName, &position, &nullable, &dataType,
			&fullType, &dflt, &comment); err != nil {
			return nil, errw(err)
		}

		colType := fullType.String
		if colType == "" {
			colType = dataType.String
		}
		baseType := dataType.String
		if baseType == "" {
			baseType = colType
		}

		cols[tblName] = append(cols[tblName], &metadata.Column{
			Name:         colName,
			Position:     position.Int64,
			BaseType:     baseType,
			ColumnType:   colType,
			Kind:         kindFromDBTypeName(log, colName, colType),
			Nullable:     !strings.EqualFold(nullable.String, "NO"),
			DefaultValue: dflt.String,
			Comment:      comment.String,
		})
	}
	return cols, errw(rows.Err())
}

// queryPrimaryKeys returns the primary key columns of the tables in schma,
// as a set of column names keyed by table name.
func queryPrimaryKeys(ctx context.Context, db sqlz.DB, info, schma string) (map[string]map[string]bool, error) {
	q := "SELECT tc.table_name, kcu.column_name FROM " + info + ".table_constraints tc " +
		"JOIN " + info + ".key_column_usage kcu " +
		"ON tc.constraint_catalog = kcu.constraint_catalog " +
		"AND tc.constraint_schema = kcu.constraint_schema " +
		"AND tc.constraint_name = kcu.constraint_name " +
		"WHERE tc.table_schema = ? AND tc.constraint_type = 'PRIMARY KEY'"
	rows, err := db.QueryContext(ctx, q, schma)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	pks := map[string]map[string]bool{}
	for rows.Next() {
		var tblName, colName string
		if err = rows.Scan(&tblName, &colName); err != nil {
			return nil, errw(err)
		}
		if pks[tblName] == nil {
			pks[tblName] = map[string]bool{}
		}
		pks[tblName][colName] = true
	}
	return pks, errw(rows.Err())
}

// queryViewDefinitions returns the view definitions of the views in schma,
// keyed by view name.
func queryViewDefinitions(ctx context.Context, db sqlz.DB, info, schma string) (map[string]string, error) {
	q := "SELECT table_name, view_definition FROM " + info + ".views WHERE table_schema = ?"
	rows, err := db.QueryContext(ctx, q, schma)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	defs := map[string]string{}
	for rows.Next() {
		var name string
		var def sql.NullString
		if err = rows.Scan(&name, &def); err != nil {
			return nil, errw(err)
		}
		defs[name] = def.String
	}
	return defs, errw(rows.Err())
}

// fillRowCounts sets RowCount on each table in tbls, via a concurrent
// SELECT COUNT(*) FROM per table or view. A failure to count a single
// table (for example, no SELECT privilege on it) is logged and the count
// is left at zero: it must not fail the whole inspection.
func fillRowCounts(ctx context.Context, db sqlz.DB, catalog, schma string, tbls []*metadata.Table) error {
	log := lg.FromContext(ctx)

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(rowCountConcurrency)
	for _, tbl := range tbls {
		g.Go(func() error {
			q := "SELECT COUNT(*) FROM " + tblfmt(tablefq.T{Catalog: catalog, Schema: schma, Table: tbl.Name})
			var count int64
			if err := db.QueryRowContext(gCtx, q).Scan(&count); err != nil {
				if gCtx.Err() != nil {
					return errw(gCtx.Err())
				}
				log.Warn("Databricks: failed to count rows; leaving row count at zero",
					lga.Table, tbl.FQName, lga.Err, err)
				return nil
			}
			tbl.RowCount = count
			return nil
		})
	}
	return g.Wait()
}

// tableTypeFromDBType maps the information_schema table_type value to
// the canonical sqlz table type. Databricks table types include MANAGED,
// EXTERNAL, FOREIGN, VIEW, MATERIALIZED_VIEW and STREAMING_TABLE. A
// streaming table is a table; a metric view is a view.
func tableTypeFromDBType(dbType string) string {
	switch strings.ToUpper(dbType) {
	case "VIEW", "METRIC_VIEW":
		return sqlz.TableTypeView
	case "MATERIALIZED_VIEW":
		return sqlz.TableTypeMaterializedView
	default:
		return sqlz.TableTypeTable
	}
}
