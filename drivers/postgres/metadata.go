package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"

	"github.com/neilotoole/sq/libsq/core/debugz"
	"github.com/neilotoole/sq/libsq/core/errz"
	"github.com/neilotoole/sq/libsq/core/kind"
	"github.com/neilotoole/sq/libsq/core/lg"
	"github.com/neilotoole/sq/libsq/core/lg/lga"
	"github.com/neilotoole/sq/libsq/core/options"
	"github.com/neilotoole/sq/libsq/core/progress"
	"github.com/neilotoole/sq/libsq/core/record"
	"github.com/neilotoole/sq/libsq/core/sqlz"
	"github.com/neilotoole/sq/libsq/core/stringz"
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// kindFromDBTypeName determines the kind.Kind from the database
// type name. For example, "VARCHAR" -> kind.Text.
// See https://www.postgresql.org/docs/9.5/datatype.html
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
	var knd kind.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	switch dbTypeName {
	default:
		log.Warn(
			"Unknown Postgres column type: using alt type",
			lga.DBType, dbTypeName,
			lga.Col, colName,
			lga.Alt, kind.Unknown,
		)
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
func setScanType(log *slog.Logger, ctd *record.ColumnTypeData, knd kind.Kind) {
	if knd == kind.Decimal {
		ctd.ScanType = sqlz.RTypeNullDecimal
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
func toNullableScanType(log *slog.Logger, colName, dbTypeName string, knd kind.Kind,
	pgScanType reflect.Type,
) reflect.Type {
	var nullableScanType reflect.Type

	switch pgScanType {
	default:
		// If we don't recognize the scan type (likely it's any),
		// we explicitly switch through the db type names that we know.
		// At this time, we will use NullString for all unrecognized
		// scan types, but nonetheless we switch through the known db type
		// names so that we see the log warning for truly unknown types.
		switch dbTypeName {
		default:
			nullableScanType = sqlz.RTypeNullString
			log.Warn(
				"Unknown Postgres scan type",
				lga.Col, colName,
				lga.ScanType, pgScanType,
				lga.DBType, dbTypeName,
				lga.Kind, knd,
				lga.DefaultTo, nullableScanType,
			)

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
		case "NUMERIC":
			nullableScanType = sqlz.RTypeNullDecimal
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

	case sqlz.RTypeDecimal:
		nullableScanType = sqlz.RTypeNullDecimal
	}

	return nullableScanType
}

// loadWithRetry runs a bulk metadata loader with vanished-relation retry, so a
// table dropped mid-scan by concurrent DDL doesn't fail the whole source scan.
func loadWithRetry[T any](ctx context.Context, load func() (T, error)) (T, error) {
	var result T
	err := doRetryVanished(ctx, func() error {
		var e error
		result, e = load()
		return e
	})
	return result, err
}

func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB, noSchema bool) (*metadata.Source, error) {
	log := lg.FromContext(ctx)
	ctx = options.NewContext(ctx, src.Options)

	md := &metadata.Source{
		Handle:   src.Handle,
		Location: src.Location,
		Driver:   src.Type,
		DBDriver: src.Type,
	}

	var schema sql.NullString
	const summaryQuery = `SELECT current_catalog, current_schema(), pg_database_size(current_catalog),
current_setting('server_version'), version(), "current_user"()`

	var size sql.NullInt64
	err := db.QueryRowContext(ctx, summaryQuery).
		Scan(&md.Name, &schema, &size, &md.DBVersion, &md.DBProduct, &md.User)
	if err != nil {
		return nil, errw(err)
	}
	if v, semverErr := parseSemver(md.DBVersion); semverErr != nil {
		lg.FromContext(ctx).Warn("Cannot derive db_semver from db_version",
			lga.Err, semverErr, lga.Version, md.DBVersion)
	} else {
		md.DBSemver = v
	}
	progress.Incr(ctx, 1)
	debugz.DebugSleep(ctx)

	if size.Valid {
		md.Size = &size.Int64
	}

	if !schema.Valid {
		return nil, errz.New("NULL value for current_schema(): check privileges and search_path")
	}

	md.Catalog = md.Name
	md.Schema = schema.String
	md.FQName = md.Name + "." + schema.String

	md.DBProperties, err = getPgSettings(ctx, db)
	if err != nil {
		return nil, err
	}

	if noSchema {
		return md, nil
	}

	tblNames, err := getAllTableNames(ctx, db)
	if err != nil {
		return nil, err
	}

	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(tuning.OptErrgroupLimit.Get(src.Options))
	tblMetas := make([]*metadata.Table, len(tblNames))
	for i := range tblNames {
		g.Go(func() error {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			var tblMeta *metadata.Table
			var mdErr error

			mdErr = doRetry(gCtx, func() error {
				tblMeta, mdErr = getTableMetadata(gCtx, db, tblNames[i])
				return mdErr
			})

			if mdErr != nil {
				switch {
				case isErrRelationDroppedMidScan(mdErr):
					// If the table is dropped while we're collecting metadata
					// (42P01, or the lower-level "could not open relation with
					// OID"), log a warning and suppress the error.
					log.Warn(
						"metadata collection: table not found (continuing regardless)",
						lga.Table, tblNames[i],
						lga.Err, mdErr,
					)
				default:
					return mdErr
				}
			}

			tblMetas[i] = tblMeta
			return nil
		})
	}

	err = g.Wait()
	if err != nil {
		return nil, errw(err)
	}

	// If a table wasn't found (possibly dropped while querying), then
	// its entry could be nil. We copy the non-nil elements to the
	// final slice.
	md.Tables = make([]*metadata.Table, 0, len(tblMetas))
	for i := range tblMetas {
		if tblMetas[i] != nil {
			md.Tables = append(md.Tables, tblMetas[i])
		}
	}

	md.RecomputeTableCounts()

	// Fetch foreign keys for all tables in one query, assign them to their
	// owning tables, and derive the cross-table back-references. Each bulk
	// loader is a single catalog query over every table in the schema; a table
	// dropped mid-query by concurrent DDL is retried (loadWithRetry) so the
	// scan doesn't fail on transient churn.
	allFKs, err := loadWithRetry(ctx, func() ([]*metadata.ForeignKey, error) {
		return getPgForeignKeys(ctx, db, "")
	})
	if err != nil {
		return nil, err
	}
	metadata.AssignForeignKeys(log, md.Tables, allFKs)

	allUCs, err := loadWithRetry(ctx, func() ([]*metadata.UniqueConstraint, error) {
		return getPgUniqueConstraints(ctx, db, "")
	})
	if err != nil {
		return nil, err
	}
	metadata.AssignUniqueConstraints(log, md.Tables, allUCs)

	allChecks, err := loadWithRetry(ctx, func() ([]*metadata.CheckConstraint, error) {
		return getPgCheckConstraints(ctx, db, "")
	})
	if err != nil {
		return nil, err
	}
	metadata.AssignCheckConstraints(log, md.Tables, allChecks)

	allTriggers, err := loadWithRetry(ctx, func() ([]*metadata.Trigger, error) {
		return getPgTriggers(ctx, db, "")
	})
	if err != nil {
		return nil, err
	}
	metadata.AssignTriggers(log, md.Tables, allTriggers)

	allIdxs, err := loadWithRetry(ctx, func() ([]*metadata.Index, error) {
		return getPgIndexes(ctx, db, "")
	})
	if err != nil {
		return nil, err
	}
	metadata.AssignIndexes(log, md.Tables, allIdxs)

	allViewDefs, err := loadWithRetry(ctx, func() (map[string]string, error) {
		return getPgViewDefinitions(ctx, db, "")
	})
	if err != nil {
		return nil, err
	}
	for _, tbl := range md.Tables {
		if tbl.TableType == sqlz.TableTypeView {
			tbl.ViewDefinition = allViewDefs[tbl.Name]
		}
	}

	// Derive incoming FK back-references last so the Assign* helpers
	// have fully populated the source. Matches the call order in the
	// other bulk-loader drivers (mysql, sqlserver, oracle, duckdb).
	metadata.LinkForeignKeys(log, md)

	return md, nil
}

func getPgSettings(ctx context.Context, db sqlz.DB) (map[string]any, error) {
	rows, err := db.QueryContext(ctx, "SELECT name, setting, vartype FROM pg_settings ORDER BY name")
	if err != nil {
		return nil, errw(err)
	}

	defer sqlz.CloseRows(lg.FromContext(ctx), rows)

	m := map[string]any{}
	for rows.Next() {
		var (
			name    string
			setting string
			typ     string
			val     any
		)
		if err = rows.Scan(&name, &setting, &typ); err != nil {
			return nil, errw(err)
		}
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		// Narrow the setting value bool, int, etc.
		val = setting
		switch typ {
		case "integer":
			var i int
			if i, err = strconv.Atoi(setting); err == nil {
				val = i
			}
		case "bool":
			var b bool
			if b, err = stringz.ParseBool(setting); err == nil {
				val = b
			}
		case "real":
			var f float64
			if f, err = strconv.ParseFloat(setting, 64); err == nil {
				val = f
			}
		case "enum", "string":
		default:
			// Leave as string
		}

		m[name] = val
	}

	if err = closeRows(rows); err != nil {
		return nil, err
	}

	return m, nil
}

// getAllTable names returns all table (or view) names in the current
// catalog & schema.
func getAllTableNames(ctx context.Context, db sqlz.DB) ([]string, error) {
	log := lg.FromContext(ctx)

	// Matviews live only in pg_catalog (pg_class.relkind='m'), not in
	// information_schema.tables, so they are UNIONed in explicitly.
	const tblNamesQuery = `SELECT table_name FROM (
  SELECT table_name FROM information_schema.tables
  WHERE table_catalog = current_catalog AND table_schema = current_schema()
  UNION
  SELECT c.relname AS table_name
  FROM pg_catalog.pg_class c
  JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
  WHERE c.relkind = 'm' AND n.nspname = current_schema()
) AS t
ORDER BY table_name`

	rows, err := db.QueryContext(ctx, tblNamesQuery)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var tblNames []string
	for rows.Next() {
		var s string
		err = rows.Scan(&s)
		if err != nil {
			return nil, errw(err)
		}
		tblNames = append(tblNames, s)
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
	}

	err = closeRows(rows)
	if err != nil {
		return nil, err
	}

	return tblNames, nil
}

func getTableMetadata(ctx context.Context, db sqlz.DB, tblName string) (*metadata.Table, error) {
	log := lg.FromContext(ctx)

	// A table name can legally contain a double-quote (e.g. `we"ird`). The
	// relation's OID is resolved via a pg_class JOIN on the raw name ($1, a
	// bind parameter) rather than a regclass text cast, which mis-parses such
	// names; the size/oid/comment functions then take the OID directly. The
	// only interpolation is the COUNT subquery's FROM, which uses a
	// double-quoted (escaped) identifier. Raw interpolation built malformed
	// SQL. See issue #1025.
	safeIdent := stringz.DoubleQuote(tblName)
	tablesQuery := `SELECT t.table_catalog, t.table_schema, t.table_name, t.table_type, t.is_insertable_into,
  (SELECT COUNT(*) FROM ` + safeIdent + `) AS table_row_count,
  pg_total_relation_size(c.oid) AS table_size,
  c.oid AS table_oid,
  obj_description(c.oid, 'pg_class') AS table_comment
FROM information_schema.tables t
JOIN pg_catalog.pg_class c ON c.relname = t.table_name
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace AND n.nspname = t.table_schema
WHERE t.table_catalog = current_database()
AND t.table_schema = current_schema()
AND t.table_name = $1`

	pgTbl := &pgTable{}
	err := db.QueryRowContext(ctx, tablesQuery, tblName).
		Scan(&pgTbl.tableCatalog, &pgTbl.tableSchema, &pgTbl.tableName, &pgTbl.tableType, &pgTbl.isInsertable,
			&pgTbl.rowCount, &pgTbl.size, &pgTbl.oid, &pgTbl.comment)
	if errors.Is(err, sql.ErrNoRows) {
		// A matview is absent from information_schema.tables, so the
		// query above returns no rows. Fall back to the pg_catalog path,
		// which also returns the canonical not-found error if tblName is
		// not a matview either.
		return getMatviewMetadata(ctx, db, tblName)
	}
	if err != nil {
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)
	debugz.DebugSleep(ctx)

	tblMeta := tblMetaFromPgTable(pgTbl)
	if tblMeta.Name != tblName {
		// Shouldn't happen, but we'll error if it does
		return nil, errz.Errorf("table {%s} not found in %s.%s", tblName, pgTbl.tableCatalog, pgTbl.tableSchema)
	}

	pgCols, err := getPgColumns(ctx, db, tblName)
	if err != nil {
		return nil, err
	}

	for _, pgCol := range pgCols {
		colMeta := colMetaFromPgColumn(log, pgCol)
		tblMeta.Columns = append(tblMeta.Columns, colMeta)
	}

	// We need to fetch the constraints to set the PK etc.
	pgConstraints, err := getPgConstraints(ctx, db, tblName)
	if err != nil {
		return nil, err
	}

	setTblMetaConstraints(log, tblMeta, pgConstraints)

	// Note: FK / unique-constraint / index loading is intentionally
	// NOT done here. getTableMetadata is called once per table from
	// getSourceMetadata's parallel errgroup, and getSourceMetadata
	// already performs a single bulk query for each of those at the
	// end. Adding per-table loads here would multiply the per-table
	// queries N times (an N+1 pattern) and the bulk-loader results
	// would just overwrite them anyway. For the per-table inspect
	// path, the wrapping Grip.TableMetadata applies the loaders with
	// a tblName filter.

	return tblMeta, nil
}

// getMatviewMetadata builds a *metadata.Table for a materialized view from
// pg_catalog. Matviews are absent from information_schema, so this is the
// fallback path from getTableMetadata. It is a two-step operation: first
// confirm that name actually is a matview (returning the canonical not-found
// error otherwise), and only then run the detail query. The size/comment/
// viewdef functions take name as a bound quote_ident($1)::regclass parameter; only the
// COUNT(*) FROM identifier is interpolated, and only after Step A confirms
// name is a real matview.
func getMatviewMetadata(ctx context.Context, db sqlz.DB, name string) (*metadata.Table, error) {
	// Step A: confirm name is a matview in the current schema, and get
	// the catalog & schema for the FQ name.
	const confirmQuery = `SELECT current_database(), current_schema()
WHERE EXISTS (
  SELECT 1 FROM pg_catalog.pg_class c
  JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
  WHERE c.relname = $1 AND n.nspname = current_schema() AND c.relkind = 'm'
)`
	var catalog, schema string
	err := db.QueryRowContext(ctx, confirmQuery, name).Scan(&catalog, &schema)
	if errors.Is(err, sql.ErrNoRows) {
		// Not a matview (nor any other relation found in info-schema):
		// behave as before for a genuinely-missing relation.
		return nil, errz.Errorf("table {%s} not found", name)
	}
	if err != nil {
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)
	debugz.DebugSleep(ctx)

	// Step B: name is confirmed a matview. The size/comment/viewdef
	// functions take name as a bound quote_ident($1)::regclass parameter. Only the
	// COUNT(*) FROM identifier must be interpolated: a relation name
	// cannot be a bind parameter in a FROM clause, and it's safe here
	// because Step A confirmed name is a real matview in this schema.
	const detailQueryTpl = `SELECT
  (SELECT COUNT(*) FROM "%s") AS row_count,
  pg_total_relation_size(quote_ident($1)::regclass) AS mv_size,
  obj_description(quote_ident($1)::regclass, 'pg_class') AS mv_comment,
  pg_get_viewdef(quote_ident($1)::regclass, true) AS view_def`
	// Double any embedded double-quotes so the identifier literal can't be broken out of.
	safeName := strings.ReplaceAll(name, `"`, `""`)
	detailQuery := fmt.Sprintf(detailQueryTpl, safeName)

	var (
		rowCount int64
		mvSize   sql.NullInt64
		comment  sql.NullString
		viewDef  sql.NullString
	)
	err = db.QueryRowContext(ctx, detailQuery, name).Scan(&rowCount, &mvSize, &comment, &viewDef)
	if err != nil {
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)
	debugz.DebugSleep(ctx)

	tblMeta := &metadata.Table{
		Name:           name,
		FQName:         fmt.Sprintf("%s.%s.%s", catalog, schema, name),
		TableType:      sqlz.TableTypeMaterializedView,
		DBTableType:    "MATERIALIZED VIEW",
		RowCount:       rowCount,
		Comment:        comment.String,
		ViewDefinition: viewDef.String,
	}
	if mvSize.Valid && mvSize.Int64 > 0 {
		tblMeta.Size = &mvSize.Int64
	}

	cols, err := getPgMatviewColumns(ctx, db, name)
	if err != nil {
		return nil, err
	}
	tblMeta.Columns = cols

	return tblMeta, nil
}

// getPgMatviewColumns returns the columns of a materialized view, sourced from
// pg_attribute (matview result columns are absent from
// information_schema.columns). Matview columns carry none of identity,
// auto-increment, generated, default, collation, or primary-key semantics.
func getPgMatviewColumns(ctx context.Context, db sqlz.DB, name string) ([]*metadata.Column, error) {
	log := lg.FromContext(ctx)

	const colsQuery = `SELECT a.attname, a.attnum,
  format_type(a.atttypid, a.atttypmod) AS data_type,
  t.typname AS udt_name,
  a.attnotnull,
  col_description(a.attrelid, a.attnum::int) AS col_comment
FROM pg_catalog.pg_attribute a
JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
WHERE c.relname = $1 AND n.nspname = current_schema() AND c.relkind = 'm'
  AND a.attnum > 0 AND NOT a.attisdropped
ORDER BY a.attnum`

	rows, err := db.QueryContext(ctx, colsQuery, name)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var cols []*metadata.Column
	for rows.Next() {
		var (
			attName, dataType, udtName string
			attNum                     int64
			attNotNull                 bool
			colComment                 sql.NullString
		)
		if err = rows.Scan(&attName, &attNum, &dataType, &udtName, &attNotNull, &colComment); err != nil {
			return nil, errw(err)
		}

		cols = append(cols, &metadata.Column{
			Name:       attName,
			Position:   attNum,
			BaseType:   udtName,
			ColumnType: dataType,
			Kind:       kindFromDBTypeName(log, attName, udtName),
			Nullable:   !attNotNull,
			Comment:    colComment.String,
		})
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
	}
	if err = closeRows(rows); err != nil {
		return nil, err
	}

	return cols, nil
}

// populateTableExtras loads outgoing FKs, incoming FKs, unique
// constraints, indexes, and (for views) the view definition for tblMeta,
// filtered by tblMeta.Name. It is the per-table counterpart to the bulk
// loaders called by getSourceMetadata, and is what Grip.TableMetadata
// uses to give single-table inspect the same shape as full-source inspect.
//
// For views, only ViewDefinition is fetched. For base tables, FKs, unique
// constraints, check constraints, triggers, and indexes are loaded.
// All other relation types are skipped.
func populateTableExtras(ctx context.Context, db sqlz.DB, tblMeta *metadata.Table) error {
	if tblMeta.TableType == sqlz.TableTypeView {
		defs, err := getPgViewDefinitions(ctx, db, tblMeta.Name)
		if err != nil {
			return err
		}
		tblMeta.ViewDefinition = defs[tblMeta.Name]
		// INSTEAD OF triggers can be attached to views; load them so that
		// per-table inspect is consistent with source-wide inspect.
		tblMeta.Triggers, err = getPgTriggers(ctx, db, tblMeta.Name)
		return err
	}
	if tblMeta.TableType == sqlz.TableTypeMaterializedView {
		// Matviews have no FK/PK/unique/check/triggers, only indexes.
		// ViewDefinition and columns were already set by getMatviewMetadata.
		var err error
		tblMeta.Indexes, err = getPgIndexes(ctx, db, tblMeta.Name)
		return err
	}
	if tblMeta.TableType != sqlz.TableTypeTable {
		return nil
	}

	outgoing, err := getPgForeignKeys(ctx, db, tblMeta.Name)
	if err != nil {
		return err
	}
	incoming, err := getPgIncomingFKs(ctx, db, tblMeta.Name)
	if err != nil {
		return err
	}
	tblMeta.FK = metadata.NewFKGroup(outgoing, incoming)

	tblMeta.UniqueConstraints, err = getPgUniqueConstraints(ctx, db, tblMeta.Name)
	if err != nil {
		return err
	}

	tblMeta.CheckConstraints, err = getPgCheckConstraints(ctx, db, tblMeta.Name)
	if err != nil {
		return err
	}

	tblMeta.Triggers, err = getPgTriggers(ctx, db, tblMeta.Name)
	if err != nil {
		return err
	}

	tblMeta.Indexes, err = getPgIndexes(ctx, db, tblMeta.Name)
	return err
}

// pgTable holds query results for table metadata.
type pgTable struct {
	tableCatalog string
	tableSchema  string
	tableName    string
	tableType    string
	oid          string
	comment      sql.NullString
	size         sql.NullInt64
	rowCount     int64
	isInsertable sqlz.NullBool // Use driver.NullBool because "YES", "NO" values
}

func tblMetaFromPgTable(pgt *pgTable) *metadata.Table {
	md := &metadata.Table{
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
	tableCatalog         string
	tableSchema          string
	tableName            string
	columnName           string
	dataType             string
	udtCatalog           string
	udtSchema            string
	udtName              string
	columnDefault        sql.NullString
	domainCatalog        sql.NullString
	domainSchema         sql.NullString
	domainName           sql.NullString
	isGenerated          sql.NullString
	collationName        sql.NullString
	generationExpression sql.NullString

	// comment holds any column comment. Note that this field is
	// not part of the standard postgres infoschema, but is
	// separately fetched.
	comment                sql.NullString
	characterMaximumLength sql.NullInt64
	characterOctetLength   sql.NullInt64
	numericPrecision       sql.NullInt64
	numericPrecisionRadix  sql.NullInt64
	numericScale           sql.NullInt64
	datetimePrecision      sql.NullInt64
	ordinalPosition        int64
	isNullable             sqlz.NullBool
	isIdentity             sqlz.NullBool
	isUpdatable            sqlz.NullBool
}

// getPgColumns queries the column metadata for tblName.
func getPgColumns(ctx context.Context, db sqlz.DB, tblName string) ([]*pgColumn, error) {
	log := lg.FromContext(ctx)

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
  collation_name,
  generation_expression,
  is_updatable,
  (
	SELECT
		pg_catalog.col_description(c.oid, cols.ordinal_position::INT)
	FROM
		pg_catalog.pg_class c
	WHERE
		c.oid = (SELECT quote_ident(cols.table_name)::regclass::oid)
		AND c.relname = cols.table_name
	) AS column_comment
FROM information_schema.columns cols
WHERE cols.table_catalog = current_catalog AND cols.table_schema = current_schema() AND cols.table_name = $1
ORDER BY cols.table_catalog, cols.table_schema, cols.table_name, cols.ordinal_position`

	rows, err := db.QueryContext(ctx, colsQuery, tblName)
	if err != nil {
		return nil, errw(err)
	}

	defer sqlz.CloseRows(log, rows)

	var cols []*pgColumn
	for rows.Next() {
		col := &pgColumn{}
		err = scanPgColumn(rows, col)
		if err != nil {
			return nil, err
		}

		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
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
		&c.isIdentity, &c.isGenerated, &c.collationName, &c.generationExpression,
		&c.isUpdatable, &c.comment)
	return errw(err)
}

func colMetaFromPgColumn(log *slog.Logger, pgCol *pgColumn) *metadata.Column {
	colMeta := &metadata.Column{
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
	colMeta.Identity = pgCol.isIdentity.Bool
	colMeta.Generated = strings.EqualFold(pgCol.isGenerated.String, "ALWAYS")
	colMeta.GeneratedExpr = pgCol.generationExpression.String
	colMeta.Collation = pgCol.collationName.String
	// Note: Postgres serial (default nextval(...)) is intentionally NOT
	// flagged as AutoIncrement; it remains visible via DefaultValue.
	return colMeta
}

// getPgConstraints returns a slice of pgConstraint. If tblName is
// empty, constraints for all tables in the current catalog & schema
// are returned. If tblName is specified, constraints just for that
// table are returned.
func getPgConstraints(ctx context.Context, db sqlz.DB, tblName string) ([]*pgConstraint, error) {
	log := lg.FromContext(ctx)

	var args []any
	query := `SELECT kcu.table_catalog,kcu.table_schema,kcu.table_name,kcu.column_name,
    kcu.ordinal_position,tc.constraint_name,tc.constraint_type,
    (
       SELECT pg_catalog.pg_get_constraintdef(pgc.oid, TRUE)
       FROM pg_catalog.pg_constraint pgc
       WHERE pgc.conrelid = (SELECT quote_ident(kcu.table_name)::regclass::oid)
       AND pgc.conname = tc.constraint_name
       limit 1
    )  AS constraint_def,
    (
       SELECT pgc.confrelid::regclass
       FROM pg_catalog.pg_constraint pgc
       WHERE pgc.conrelid = (SELECT quote_ident(kcu.table_name)::regclass::oid)
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
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var constraints []*pgConstraint

	for rows.Next() {
		pgc := &pgConstraint{}
		err = rows.Scan(&pgc.tableCatalog, &pgc.tableSchema, &pgc.tableName, &pgc.columnName, &pgc.ordinalPosition,
			&pgc.constraintName, &pgc.constraintType, &pgc.constraintDef, &pgc.constraintFKeyTableName)
		if err != nil {
			return nil, errw(err)
		}

		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
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
	tableCatalog string
	tableSchema  string
	tableName    string
	columnName   string

	constraintName sql.NullString
	constraintType sql.NullString
	constraintDef  sql.NullString

	// constraintFKeyTableName holds the name of the table to which
	// a foreign-key constraint points to. This is null if this
	// constraint is not a foreign key.
	constraintFKeyTableName sql.NullString
	ordinalPosition         int64
}

// setTblMetaConstraints updates tblMeta with constraints found
// in pgConstraints.
func setTblMetaConstraints(log *slog.Logger, tblMeta *metadata.Table, pgConstraints []*pgConstraint) {
	for _, pgc := range pgConstraints {
		fqTblName := pgc.tableCatalog + "." + pgc.tableSchema + "." + pgc.tableName
		if fqTblName != tblMeta.FQName {
			continue
		}

		if pgc.constraintType.String == constraintTypePK {
			colMeta := tblMeta.Column(pgc.columnName)
			if colMeta == nil {
				// Shouldn't happen
				log.Warn(
					"No column found matching constraint",
					lga.Target, tblMeta.Name+"."+pgc.columnName,
					"constraint", pgc.constraintName,
				)
				continue
			}
			colMeta.PrimaryKey = true
		}
	}
}

const constraintTypePK = "PRIMARY KEY"

// getPgUniqueConstraints returns the UNIQUE constraints declared on
// tables in the current catalog and schema. If tblName is empty,
// constraints for every table in the current schema are returned;
// otherwise only constraints on tblName are returned. Composite
// constraints are collapsed into a single UniqueConstraint with
// Columns ordered by ordinal_position.
func getPgUniqueConstraints(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.UniqueConstraint, error) {
	log := lg.FromContext(ctx)

	query := `SELECT
  tc.constraint_name,
  tc.table_name,
  kcu.column_name,
  kcu.ordinal_position
FROM information_schema.table_constraints AS tc
JOIN information_schema.key_column_usage  AS kcu
  ON  kcu.constraint_catalog = tc.constraint_catalog
  AND kcu.constraint_schema  = tc.constraint_schema
  AND kcu.constraint_name    = tc.constraint_name
WHERE tc.constraint_type = 'UNIQUE'
  AND tc.table_catalog = current_catalog
  AND tc.table_schema  = current_schema()
`
	var args []any
	if tblName != "" {
		query += ` AND tc.table_name = $1`
		args = append(args, tblName)
	}
	query += ` ORDER BY tc.table_name, tc.constraint_name, kcu.ordinal_position`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type ucKey struct {
		table, name string
	}
	byKey := map[ucKey]*metadata.UniqueConstraint{}
	var ucs []*metadata.UniqueConstraint
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		var (
			constraintName, ownerTable, columnName string
			ordinalPosition                        int64
		)
		if err = rows.Scan(&constraintName, &ownerTable, &columnName, &ordinalPosition); err != nil {
			return nil, errw(err)
		}
		k := ucKey{table: ownerTable, name: constraintName}
		uc, ok := byKey[k]
		if !ok {
			uc = &metadata.UniqueConstraint{
				Name:  constraintName,
				Table: ownerTable,
			}
			byKey[k] = uc
			ucs = append(ucs, uc)
		}
		uc.Columns = append(uc.Columns, columnName)
	}
	return ucs, errw(rows.Err())
}

// getPgCheckConstraints returns the CHECK constraints declared on tables in
// the current catalog and schema. If tblName is empty, constraints for every
// table in the current schema are returned; otherwise only constraints on
// tblName are returned. The Clause field holds the engine-formatted expression
// text as returned by pg_get_constraintdef.
func getPgCheckConstraints(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.CheckConstraint, error) {
	log := lg.FromContext(ctx)

	query := `SELECT rel.relname, con.conname, pg_get_constraintdef(con.oid, TRUE)
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_class rel ON rel.oid = con.conrelid
JOIN pg_catalog.pg_namespace n ON n.oid = rel.relnamespace
WHERE con.contype = 'c' AND n.nspname = current_schema()`

	var args []any
	if tblName != "" {
		query += ` AND rel.relname = $1`
		args = append(args, tblName)
	}
	query += ` ORDER BY rel.relname, con.conname`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var checks []*metadata.CheckConstraint
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		cc := &metadata.CheckConstraint{}
		if err = rows.Scan(&cc.Table, &cc.Name, &cc.Clause); err != nil {
			return nil, errw(err)
		}
		checks = append(checks, cc)
	}
	return checks, errw(rows.Err())
}

// getPgTriggers returns the triggers declared on tables (and views) in the
// current schema. If tblName is empty, triggers for every relation in the
// current schema are returned; otherwise only triggers on tblName are returned.
// Internal triggers (constraint-enforcement triggers created by Postgres itself)
// are excluded via NOT t.tgisinternal.
//
// Note: INSTEAD OF triggers can only be defined on views. Both paths correctly
// capture them: the source-wide path (tblName == "") uses an unfiltered query so
// AssignTriggers populates every entry in md.Tables including views; the
// per-table path in populateTableExtras explicitly calls getPgTriggers when
// TableType is view, so single-table inspect of a view returns its INSTEAD OF
// triggers as well.
func getPgTriggers(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.Trigger, error) {
	log := lg.FromContext(ctx)

	query := `SELECT c.relname AS table_name, t.tgname,
  CASE WHEN (t.tgtype & 2)<>0 THEN 'BEFORE'
       WHEN (t.tgtype & 64)<>0 THEN 'INSTEAD OF' ELSE 'AFTER' END AS timing,
  (t.tgtype & 4)<>0  AS on_insert,
  (t.tgtype & 8)<>0  AS on_delete,
  (t.tgtype & 16)<>0 AS on_update,
  t.tgenabled <> 'D' AS enabled,
  pg_get_triggerdef(t.oid, TRUE) AS definition
FROM pg_trigger t
JOIN pg_class c ON c.oid = t.tgrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE NOT t.tgisinternal AND n.nspname = current_schema()`

	var args []any
	if tblName != "" {
		query += ` AND c.relname = $1`
		args = append(args, tblName)
	}
	query += ` ORDER BY c.relname, t.tgname`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	var triggers []*metadata.Trigger
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		var (
			tblNameVal string
			trigName   string
			timing     string
			onInsert   bool
			onDelete   bool
			onUpdate   bool
			enabled    bool
			definition string
		)
		if err = rows.Scan(&tblNameVal, &trigName, &timing,
			&onInsert, &onDelete, &onUpdate, &enabled, &definition); err != nil {
			return nil, errw(err)
		}

		var events []string
		if onInsert {
			events = append(events, "INSERT")
		}
		if onUpdate {
			events = append(events, "UPDATE")
		}
		if onDelete {
			events = append(events, "DELETE")
		}

		// Allocate a fresh bool inside the loop so each Trigger.Enabled
		// points to its own value and not a shared loop variable.
		enabledVal := enabled
		triggers = append(triggers, &metadata.Trigger{
			Name:       trigName,
			Table:      tblNameVal,
			Timing:     timing,
			Events:     events,
			Enabled:    &enabledVal,
			Definition: definition,
		})
	}
	return triggers, errw(rows.Err())
}

// getPgViewDefinitions returns a map of view name → defining SQL for regular
// views (relkind='v') in the current schema. If tblName is non-empty, only
// that view is returned; passing an empty tblName returns all views.
//
// The definition is produced by pg_get_viewdef(oid, true), which formats the
// SQL with pretty-printing enabled. Base tables are never included; callers
// should only read from the returned map for view-typed relations.
func getPgViewDefinitions(ctx context.Context, db sqlz.DB, tblName string) (map[string]string, error) {
	log := lg.FromContext(ctx)

	query := `SELECT c.relname, pg_get_viewdef(c.oid, true)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'v' AND n.nspname = current_schema()`

	var args []any
	if tblName != "" {
		query += ` AND c.relname = $1`
		args = append(args, tblName)
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	defs := map[string]string{}
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		var (
			name string
			def  sql.NullString
		)
		if err = rows.Scan(&name, &def); err != nil {
			return nil, errw(err)
		}
		defs[name] = strings.TrimSpace(def.String)
	}
	return defs, errw(rows.Err())
}

// pgIndexHasIndnkeyatts reports whether pg_index has the indnkeyatts
// column. The column was added in Postgres 11 alongside INCLUDE-column
// support; older versions (PG 9.x / 10.x) don't expose it and would
// fail to parse a query that references it.
func pgIndexHasIndnkeyatts(ctx context.Context, db sqlz.DB) (bool, error) {
	const q = `SELECT EXISTS (
  SELECT 1 FROM pg_attribute
  WHERE attrelid = 'pg_catalog.pg_index'::regclass
    AND attname  = 'indnkeyatts'
    AND NOT attisdropped
)`
	var ok bool
	if err := db.QueryRowContext(ctx, q).Scan(&ok); err != nil {
		return false, errw(err)
	}
	return ok, nil
}

// getPgIndexes returns the physical indexes declared on tables in the
// current schema. If tblName is empty, indexes for every table are
// returned; otherwise only indexes on tblName are returned.
//
// pg_index.indkey is an int2vector of column attnums; unnest() WITH
// ORDINALITY preserves the original key order when joining to
// pg_attribute. Functional/expression indexes contribute rows with
// attnum=0 (not present in pg_attribute); the LEFT JOIN yields a NULL
// attname for those positions, which is stored as the empty-string
// sentinel so Index.Columns preserves key arity. Indexes whose every
// key position is an expression are omitted entirely.
//
// INCLUDE columns (Postgres 11+) are stored after the key columns in
// indkey; restricting to ord <= ix.indnkeyatts keeps Index.Columns as
// key-only, matching the contract documented on the type and the
// SQL Server loader (which excludes INCLUDE columns explicitly).
// pg_index.indnkeyatts itself only exists on PG 11+, so the filter is
// added at runtime based on column-existence detection — PG 9/10 don't
// have INCLUDE columns, so the filter is a no-op there.
//
// ix.indisvalid filters out indexes that are being built (`CREATE INDEX
// CONCURRENTLY` in progress) or failed; it has been part of pg_index
// since Postgres 8.0, so it's safe for every sq-supported version.
// We deliberately don't use ix.indislive here — that flag was only
// added in PG 9.3 and would prevent the query from parsing on older
// servers.
func getPgIndexes(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.Index, error) {
	log := lg.FromContext(ctx)

	hasIndnkeyatts, err := pgIndexHasIndnkeyatts(ctx, db)
	if err != nil {
		return nil, err
	}

	keyOnlyFilter := ""
	if hasIndnkeyatts {
		keyOnlyFilter = `
  AND k.ord <= ix.indnkeyatts`
	}

	query := `SELECT
  t.relname        AS table_name,
  c.relname        AS index_name,
  ix.indisunique   AS is_unique,
  ix.indisprimary  AS is_primary,
  am.amname        AS index_type,
  attr.attname     AS column_name,
  k.ord            AS ordinal_position
FROM pg_index            AS ix
JOIN pg_class            AS c    ON c.oid = ix.indexrelid
JOIN pg_class            AS t    ON t.oid = ix.indrelid
JOIN pg_namespace        AS ns   ON ns.oid = c.relnamespace
JOIN pg_am               AS am   ON am.oid = c.relam
JOIN LATERAL unnest(ix.indkey::int2[]) WITH ORDINALITY AS k(attnum, ord) ON TRUE
LEFT JOIN pg_attribute   AS attr ON attr.attrelid = t.oid AND attr.attnum = k.attnum
WHERE ns.nspname = current_schema()
  AND t.relkind IN ('r', 'p', 'm')
  AND ix.indisvalid` + keyOnlyFilter + `
`
	var args []any
	if tblName != "" {
		query += ` AND t.relname = $1`
		args = append(args, tblName)
	}
	query += ` ORDER BY t.relname, c.relname, k.ord`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type idxKey struct {
		table, name string
	}
	byKey := map[idxKey]*metadata.Index{}
	var indexes []*metadata.Index
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		var (
			tableName, indexName, indexType string
			columnName                      sql.NullString
			isUnique, isPrimary             bool
			ordinalPosition                 int64
		)
		if err = rows.Scan(&tableName, &indexName, &isUnique, &isPrimary,
			&indexType, &columnName, &ordinalPosition); err != nil {
			return nil, errw(err)
		}
		k := idxKey{table: tableName, name: indexName}
		idx, ok := byKey[k]
		if !ok {
			idx = &metadata.Index{
				Name:    indexName,
				Table:   tableName,
				Unique:  isUnique,
				Primary: isPrimary,
				Type:    strings.ToUpper(indexType),
			}
			byKey[k] = idx
			indexes = append(indexes, idx)
		}
		// attnum=0 in pg_index.indkey marks an expression key position;
		// the LEFT JOIN yields a NULL attname there. Record the
		// empty-string sentinel (columnName.String == "") so Columns
		// preserves the index's key arity. See metadata.Index.Columns.
		idx.Columns = append(idx.Columns, columnName.String)
	}
	if err := rows.Err(); err != nil {
		return nil, errw(err)
	}
	// Omit indexes whose key positions are all expressions.
	kept := indexes[:0]
	for _, idx := range indexes {
		if metadata.AllExpressionKeys(idx.Columns) {
			log.Debug("postgres: dropping index with only expression keys",
				"table", idx.Table, "index", idx.Name)
			continue
		}
		kept = append(kept, idx)
	}
	return kept, nil
}

// getPgForeignKeys returns the outgoing foreign-key constraints in the
// current catalog and schema. If tblName is empty, FKs for every table
// in the current schema are returned; otherwise only FKs declared on
// tblName are returned.
//
// Composite foreign keys are returned as a single ForeignKey whose
// Columns / RefColumns slices are ordered by the FK's column position.
//
// Cross-table linking (Table.FK.Incoming) is not done here; callers
// must invoke [metadata.LinkForeignKeys] on the owning
// [metadata.Source] after assigning FKs to tables.
func getPgForeignKeys(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)

	// referential_constraints joined twice to key_column_usage: once
	// for the referencing (FK) side and once for the referenced (PK)
	// side. ccu.ordinal_position is matched to kcu.position_in_unique_constraint
	// so composite keys line up positionally.
	// NULLIF clears RefCatalog / RefSchema when the reference is in the
	// current catalog / schema. This way the per-table inspect path
	// produces the same normalized shape that source-level inspect
	// gets from [metadata.LinkForeignKeys], without needing a Go-side
	// post-processing pass.
	query := `SELECT
  rc.constraint_name,
  kcu.table_name                                       AS fk_table,
  kcu.column_name                                      AS fk_column,
  kcu.ordinal_position,
  NULLIF(ccu.table_catalog, current_catalog)           AS ref_catalog,
  NULLIF(ccu.table_schema, current_schema())           AS ref_schema,
  ccu.table_name                                       AS ref_table,
  ccu.column_name                                      AS ref_column,
  rc.delete_rule,
  rc.update_rule
FROM information_schema.referential_constraints AS rc
JOIN information_schema.key_column_usage         AS kcu
  ON  kcu.constraint_catalog = rc.constraint_catalog
  AND kcu.constraint_schema  = rc.constraint_schema
  AND kcu.constraint_name    = rc.constraint_name
JOIN information_schema.key_column_usage         AS ccu
  ON  ccu.constraint_catalog = rc.unique_constraint_catalog
  AND ccu.constraint_schema  = rc.unique_constraint_schema
  AND ccu.constraint_name    = rc.unique_constraint_name
  AND ccu.ordinal_position   = kcu.position_in_unique_constraint
WHERE kcu.table_catalog = current_catalog
  AND kcu.table_schema  = current_schema()
`
	var args []any
	if tblName != "" {
		query += ` AND kcu.table_name = $1`
		args = append(args, tblName)
	}
	query += ` ORDER BY kcu.table_name, rc.constraint_name, kcu.ordinal_position`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type fkKey struct {
		table, name string
	}
	byKey := map[fkKey]*metadata.ForeignKey{}
	var fks []*metadata.ForeignKey
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		var (
			constraintName, fkTable, fkColumn string
			refTable, refCol                  string
			refCatalog, refSchema             sql.NullString
			deleteRule, updateRule            sql.NullString
			ordinalPosition                   int64
		)
		if err = rows.Scan(&constraintName, &fkTable, &fkColumn, &ordinalPosition,
			&refCatalog, &refSchema, &refTable, &refCol, &deleteRule, &updateRule); err != nil {
			return nil, errw(err)
		}

		k := fkKey{table: fkTable, name: constraintName}
		fk, ok := byKey[k]
		if !ok {
			fk = &metadata.ForeignKey{
				Name:       constraintName,
				Table:      fkTable,
				RefCatalog: refCatalog.String,
				RefSchema:  refSchema.String,
				RefTable:   refTable,
				OnDelete:   deleteRule.String,
				OnUpdate:   updateRule.String,
			}
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, fkColumn)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	if err = closeRows(rows); err != nil {
		return nil, err
	}
	return fks, nil
}

// getPgIncomingFKs returns the foreign-key constraints declared on
// other tables in the current schema whose referenced side is tblName.
// The same query shape as getPgForeignKeys, but filtered on the
// referenced (ccu) side rather than the referencing (kcu) side. Used
// by the single-table inspect path to populate [Table.FK].Incoming
// without walking every other table.
func getPgIncomingFKs(ctx context.Context, db sqlz.DB, tblName string) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)

	const query = `SELECT
  rc.constraint_name,
  kcu.table_name              AS fk_table,
  kcu.column_name             AS fk_column,
  kcu.ordinal_position,
  ccu.table_name              AS ref_table,
  ccu.column_name             AS ref_column,
  rc.delete_rule,
  rc.update_rule
FROM information_schema.referential_constraints AS rc
JOIN information_schema.key_column_usage         AS kcu
  ON  kcu.constraint_catalog = rc.constraint_catalog
  AND kcu.constraint_schema  = rc.constraint_schema
  AND kcu.constraint_name    = rc.constraint_name
JOIN information_schema.key_column_usage         AS ccu
  ON  ccu.constraint_catalog = rc.unique_constraint_catalog
  AND ccu.constraint_schema  = rc.unique_constraint_schema
  AND ccu.constraint_name    = rc.unique_constraint_name
  AND ccu.ordinal_position   = kcu.position_in_unique_constraint
WHERE ccu.table_catalog = current_catalog
  AND ccu.table_schema  = current_schema()
  AND ccu.table_name    = $1
ORDER BY kcu.table_name, rc.constraint_name, kcu.ordinal_position`

	rows, err := db.QueryContext(ctx, query, tblName)
	if err != nil {
		return nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	type fkKey struct {
		table, name string
	}
	byKey := map[fkKey]*metadata.ForeignKey{}
	var fks []*metadata.ForeignKey
	for rows.Next() {
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		var (
			constraintName, fkTable, fkColumn string
			refTable, refCol                  string
			deleteRule, updateRule            sql.NullString
			ordinalPosition                   int64
		)
		if err = rows.Scan(&constraintName, &fkTable, &fkColumn, &ordinalPosition,
			&refTable, &refCol, &deleteRule, &updateRule); err != nil {
			return nil, errw(err)
		}

		k := fkKey{table: fkTable, name: constraintName}
		fk, ok := byKey[k]
		if !ok {
			fk = &metadata.ForeignKey{
				Name:     constraintName,
				Table:    fkTable,
				RefTable: refTable,
				OnDelete: deleteRule.String,
				OnUpdate: updateRule.String,
			}
			// RefCatalog/RefSchema are left empty: the referenced
			// table is the current one we're inspecting, which by
			// construction lives in current_catalog / current_schema,
			// so omitting matches the in-source convention used by
			// [metadata.LinkForeignKeys].
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, fkColumn)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	if err = closeRows(rows); err != nil {
		return nil, err
	}
	return fks, nil
}

// closeRows invokes rows.Err and rows.Close, returning
// an error if either of those methods returned an error.
func closeRows(rows *sql.Rows) error {
	if rows == nil {
		return nil
	}
	err1 := rows.Err()
	err2 := rows.Close()
	if err1 != nil {
		return errw(err1)
	}
	return errw(err2)
}

// semverRx matches a leading dotted-numeric version token (up to three parts).
var semverRx = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// parseSemver normalizes a Postgres server_version string to canonical semver
// (e.g. "v16.1.0"). Since PG 10 the version is two-part (major.minor); the
// leading token also handles any trailing distro parenthetical.
func parseSemver(raw string) (string, error) {
	m := semverRx.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", errz.Errorf("no semver in postgres version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid postgres semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver.
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT current_setting('server_version')").Scan(&raw); err != nil {
		return "", errw(err)
	}
	return parseSemver(raw)
}
