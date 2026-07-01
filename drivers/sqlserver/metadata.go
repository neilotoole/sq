package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/c2h5oh/datasize"
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
	"github.com/neilotoole/sq/libsq/core/tuning"
	"github.com/neilotoole/sq/libsq/source"
	"github.com/neilotoole/sq/libsq/source/drivertype"
	"github.com/neilotoole/sq/libsq/source/metadata"
)

// kindFromDBTypeName determines the kind.Kind from the database
// type name. For example, "VARCHAR" -> kind.Text.
func kindFromDBTypeName(log *slog.Logger, colName, dbTypeName string) kind.Kind {
	var knd kind.Kind
	dbTypeName = strings.ToUpper(dbTypeName)

	switch dbTypeName {
	default:
		log.Warn("Unknown SQLServer database type for column: using default kind",
			"db_type", dbTypeName, lga.Col, colName, lga.Kind, kind.Unknown)
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
// Most importantly, if ct is nullable column, we set colTypeData.ScanType
// to a nullable type. This is because the driver doesn't report nullable
// scan types.
func setScanType(ct *record.ColumnTypeData, knd kind.Kind) {
	if knd == kind.Decimal {
		// The driver wants us to use []byte for DECIMAL, so
		// we override that here to use decimal.Decimal.
		if ct.Nullable {
			ct.ScanType = sqlz.RTypeNullDecimal
		} else {
			ct.ScanType = sqlz.RTypeDecimal
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

func getSourceMetadata(ctx context.Context, src *source.Source, db sqlz.DB, noSchema bool) (*metadata.Source, error) {
	log := lg.FromContext(ctx)
	ctx = options.NewContext(ctx, src.Options)

	const query = `SELECT DB_NAME(), SCHEMA_NAME(), SERVERPROPERTY('ProductVersion'), @@VERSION,
(SELECT SUM(size) * 8192
FROM sys.master_files WITH(NOWAIT)
WHERE database_id = DB_ID()
GROUP BY database_id) AS total_size_bytes`

	md := &metadata.Source{Driver: drivertype.MSSQL, DBDriver: drivertype.MSSQL}
	md.Handle = src.Handle
	md.Location = src.Location

	var catalog, schema string
	var size sql.NullInt64
	err := db.QueryRowContext(ctx, query).
		Scan(&catalog, &schema, &md.DBVersion, &md.DBProduct, &size)
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

	md.Name = catalog
	md.FQName = catalog + "." + schema
	md.Catalog = catalog
	md.Schema = schema

	if md.DBProperties, err = getDBProperties(ctx, db); err != nil {
		return nil, err
	}

	if noSchema {
		return md, nil
	}

	tblNames, tblTypes, err := getAllTables(ctx, db, schema)
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

			tblMeta, gErr := getTableMetadata(gCtx, db, catalog, schema, tblNames[i], tblTypes[i], false)
			if gErr != nil {
				if hasErrCode(gErr, errCodeObjectNotExist) {
					// This can happen if the table is dropped while
					// we're collecting metadata. We log a warning and continue.
					log.Warn(
						"Table metadata: table not found (continuing regardless)",
						lga.Table, tblNames[i],
						lga.Err, gErr,
					)

					return nil
				}

				return gErr
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

	// Fetch FKs / unique constraints / indexes / check constraints / triggers
	// in bulk queries rather than N per-table calls inside the errgroup above.
	// The Assign* helpers route each result to its owning table by name,
	// and LinkForeignKeys derives FK.Incoming across the whole source.
	allFKs, err := getMSSQLForeignKeys(ctx, db, catalog, schema, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignForeignKeys(log, md.Tables, allFKs)

	allUCs, err := getMSSQLUniqueConstraints(ctx, db, catalog, schema, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignUniqueConstraints(log, md.Tables, allUCs)

	allChecks, err := getMSSQLCheckConstraints(ctx, db, schema, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignCheckConstraints(log, md.Tables, allChecks)

	allTriggers, err := getMSSQLTriggers(ctx, db, schema, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignTriggers(log, md.Tables, allTriggers)

	allIdxs, err := getMSSQLIndexes(ctx, db, schema, "")
	if err != nil {
		return nil, err
	}
	metadata.AssignIndexes(log, md.Tables, allIdxs)

	allViewDefs, err := getMSSQLViewDefinitions(ctx, db, schema, "")
	if err != nil {
		return nil, err
	}
	for _, tbl := range md.Tables {
		if tbl.TableType == sqlz.TableTypeView {
			tbl.ViewDefinition = allViewDefs[tbl.Name]
		}
	}

	metadata.LinkForeignKeys(log, md)

	return md, nil
}

// getTableMetadata builds the [metadata.Table] for a single
// (catalog, schema, table). The loadConstraints flag controls whether
// per-table FK / unique-constraint / check-constraint / trigger / index /
// view-definition queries are issued:
//
//   - Source-level inspect passes false. [getSourceMetadata] runs
//     bulk loaders after the per-table errgroup completes, which is a
//     constant number of round-trips instead of N. [metadata.LinkForeignKeys]
//     then derives [FK.Incoming] across the whole source.
//   - Single-table inspect (grip.TableMetadata) passes true so the
//     standalone [metadata.Table] carries its full metadata directly,
//     including [FK.Incoming].
func getTableMetadata(ctx context.Context, db sqlz.DB, tblCatalog,
	tblSchema, tblName, tblType string, loadConstraints bool,
) (*metadata.Table, error) {
	const tplTblUsage = `sp_spaceused '%s'`

	tblMeta := &metadata.Table{Name: tblName, DBTableType: tblType}
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

	// REVISIT: This error can occur:
	//
	//  sql: Scan error on column index 0, name "name": converting NULL to string is unsupported
	//
	// We should probably use sql.NullString? This situation can arise if the table
	// is deleted while this process is taking place. Maybe wrap the entire thing
	// in a transaction? Or figure out how to fail more gracefully?

	err := row.Scan(&tblMeta.Name, &rowCount, &reserved, &data, &indexSize, &unused)
	if err != nil {
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)
	debugz.DebugSleep(ctx)

	if rowCount.Valid {
		tblMeta.RowCount, err = strconv.ParseInt(strings.TrimSpace(rowCount.String), 10, 64)
		if err != nil {
			return nil, errw(err)
		}
	} else {
		// We can't get the "row count" for a VIEW from sp_spaceused,
		// so we need to select it the old-fashioned way.
		err = db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %q", tblName)).Scan(&tblMeta.RowCount)
		if err != nil {
			return nil, errw(err)
		}
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
	}

	if reserved.Valid {
		var byteCount datasize.ByteSize
		err = byteCount.UnmarshalText([]byte(reserved.String))
		if err != nil {
			return nil, errw(err)
		}
		size := int64(byteCount.Bytes()) //nolint:gosec
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
		return nil, errw(err)
	}

	cols := make([]*metadata.Column, len(dbCols))
	for i := range dbCols {
		cols[i] = &metadata.Column{
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

		cols[i].Identity = dbCols[i].IsIdentity
		cols[i].Generated = dbCols[i].IsComputed
		cols[i].GeneratedExpr = dbCols[i].GeneratedExpr.String
		cols[i].Collation = dbCols[i].CollationName.String
		// SQL Server IDENTITY maps to Identity; there is no separate
		// auto-increment concept, so AutoIncrement remains false.

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

	// Source-level inspect skips per-table constraint/view queries entirely;
	// [getSourceMetadata] runs bulk loaders after the errgroup completes.
	if !loadConstraints {
		return tblMeta, nil
	}

	// For views: fetch the definition and any INSTEAD OF triggers, then return.
	// No FKs, UCs, indexes, or check constraints apply.
	if tblMeta.TableType == sqlz.TableTypeView {
		defs, vErr := getMSSQLViewDefinitions(ctx, db, tblSchema, tblName)
		if vErr != nil {
			return nil, vErr
		}
		tblMeta.ViewDefinition = defs[tblName]
		// INSTEAD OF triggers on views carry parent_class=1 (same as table
		// DML triggers), so getMSSQLTriggers returns them when called for a view.
		tblMeta.Triggers, vErr = getMSSQLTriggers(ctx, db, tblSchema, tblName)
		if vErr != nil {
			return nil, vErr
		}
		return tblMeta, nil
	}

	// Below: base tables only (loadConstraints=true, TableType=TABLE).
	if tblMeta.TableType != sqlz.TableTypeTable {
		return tblMeta, nil
	}

	outgoing, err := getMSSQLForeignKeys(ctx, db, tblCatalog, tblSchema, tblName)
	if err != nil {
		return nil, err
	}
	incoming, err := getMSSQLIncomingFKs(ctx, db, tblSchema, tblName)
	if err != nil {
		return nil, err
	}
	tblMeta.FK = metadata.NewFKGroup(outgoing, incoming)

	tblMeta.UniqueConstraints, err = getMSSQLUniqueConstraints(ctx, db, tblCatalog, tblSchema, tblName)
	if err != nil {
		return nil, err
	}

	tblMeta.CheckConstraints, err = getMSSQLCheckConstraints(ctx, db, tblSchema, tblName)
	if err != nil {
		return nil, err
	}

	tblMeta.Triggers, err = getMSSQLTriggers(ctx, db, tblSchema, tblName)
	if err != nil {
		return nil, err
	}

	tblMeta.Indexes, err = getMSSQLIndexes(ctx, db, tblSchema, tblName)
	if err != nil {
		return nil, err
	}

	return tblMeta, nil
}

// getMSSQLUniqueConstraints returns the UNIQUE constraints declared on
// tables in the given catalog and schema. If tblName is empty,
// constraints for every table in the schema are returned; otherwise
// only constraints on tblName are returned.
func getMSSQLUniqueConstraints(ctx context.Context, db sqlz.DB, tblCatalog, tblSchema, tblName string,
) ([]*metadata.UniqueConstraint, error) {
	log := lg.FromContext(ctx)

	query := `SELECT
  tc.CONSTRAINT_NAME,
  tc.TABLE_NAME,
  kcu.COLUMN_NAME,
  kcu.ORDINAL_POSITION
FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS AS tc
JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE  AS kcu
  ON  kcu.CONSTRAINT_CATALOG = tc.CONSTRAINT_CATALOG
  AND kcu.CONSTRAINT_SCHEMA  = tc.CONSTRAINT_SCHEMA
  AND kcu.CONSTRAINT_NAME    = tc.CONSTRAINT_NAME
WHERE tc.CONSTRAINT_TYPE = 'UNIQUE'
  AND tc.TABLE_CATALOG = @p1
  AND tc.TABLE_SCHEMA  = @p2
`
	args := []any{tblCatalog, tblSchema}
	if tblName != "" {
		query += ` AND tc.TABLE_NAME = @p3`
		args = append(args, tblName)
	}
	query += ` ORDER BY tc.TABLE_NAME, tc.CONSTRAINT_NAME, kcu.ORDINAL_POSITION`

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

// getMSSQLIndexes returns the physical indexes declared on tables in
// the given schema. If tblName is empty, indexes for every table in
// the schema are returned. Heap (type=0) entries and INCLUDE columns
// are excluded.
func getMSSQLIndexes(ctx context.Context, db sqlz.DB, tblSchema, tblName string) ([]*metadata.Index, error) {
	log := lg.FromContext(ctx)

	query := `SELECT
  t.name           AS table_name,
  i.name           AS index_name,
  i.is_unique,
  i.is_primary_key,
  i.type_desc      AS index_type,
  c.name           AS column_name,
  ic.key_ordinal
FROM sys.indexes        AS i
JOIN sys.tables         AS t  ON t.object_id = i.object_id
JOIN sys.schemas        AS s  ON s.schema_id = t.schema_id
JOIN sys.index_columns  AS ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
JOIN sys.columns        AS c  ON c.object_id  = ic.object_id AND c.column_id = ic.column_id
WHERE s.name = @p1
  AND i.type > 0
  AND ic.is_included_column = 0
`
	args := []any{tblSchema}
	if tblName != "" {
		query += ` AND t.name = @p2`
		args = append(args, tblName)
	}
	query += ` ORDER BY t.name, i.name, ic.key_ordinal`

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
			tableName, indexName, indexType, columnName string
			isUnique, isPrimary                         bool
			keyOrdinal                                  int64
		)
		if err = rows.Scan(&tableName, &indexName, &isUnique, &isPrimary,
			&indexType, &columnName, &keyOrdinal); err != nil {
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
				Type:    indexType,
			}
			byKey[k] = idx
			indexes = append(indexes, idx)
		}
		idx.Columns = append(idx.Columns, columnName)
	}
	return indexes, errw(rows.Err())
}

// getMSSQLForeignKeys returns the outgoing foreign keys for tables in
// the given catalog and schema. If tblName is empty, FKs for every
// table in the schema are returned; otherwise only FKs declared on
// tblName are returned. Composite foreign keys are collapsed into a
// single ForeignKey ordered by constraint_column_id.
//
// SQL Server's INFORMATION_SCHEMA.KEY_COLUMN_USAGE does not expose
// POSITION_IN_UNIQUE_CONSTRAINT, so this loader queries the sys.*
// catalog views directly (sys.foreign_keys + sys.foreign_key_columns
// joined to sys.objects / sys.columns / sys.schemas).
//
// Cross-table linking (Table.FK.Incoming) is not performed here;
// callers must invoke metadata.LinkForeignKeys at the source level.
func getMSSQLForeignKeys(ctx context.Context, db sqlz.DB, _ /*tblCatalog*/, tblSchema, tblName string,
) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)

	// fk.delete_referential_action_desc / update_referential_action_desc
	// use underscored values ('NO_ACTION', 'SET_NULL'). REPLACE rewrites
	// them to the space-separated form used by the other drivers
	// ('NO ACTION', 'SET NULL'). NULLIF clears ref_schema when the
	// referenced table is in the same schema, matching the
	// normalization that metadata.LinkForeignKeys applies at the source
	// level.
	query := `SELECT
  fk.name                                                AS constraint_name,
  parent_t.name                                          AS fk_table,
  parent_c.name                                          AS fk_column,
  fkc.constraint_column_id                               AS ordinal_position,
  NULLIF(ref_s.name, @p1)                                AS ref_schema,
  ref_t.name                                             AS ref_table,
  ref_c.name                                             AS ref_column,
  REPLACE(fk.delete_referential_action_desc, '_', ' ')   AS delete_rule,
  REPLACE(fk.update_referential_action_desc, '_', ' ')   AS update_rule
FROM sys.foreign_keys        AS fk
JOIN sys.foreign_key_columns AS fkc      ON fkc.constraint_object_id = fk.object_id
JOIN sys.objects             AS parent_t ON parent_t.object_id       = fkc.parent_object_id
JOIN sys.schemas             AS parent_s ON parent_s.schema_id       = parent_t.schema_id
JOIN sys.columns             AS parent_c ON parent_c.object_id       = fkc.parent_object_id
                                        AND parent_c.column_id       = fkc.parent_column_id
JOIN sys.objects             AS ref_t    ON ref_t.object_id          = fkc.referenced_object_id
JOIN sys.schemas             AS ref_s    ON ref_s.schema_id          = ref_t.schema_id
JOIN sys.columns             AS ref_c    ON ref_c.object_id          = fkc.referenced_object_id
                                        AND ref_c.column_id          = fkc.referenced_column_id
WHERE parent_s.name = @p1
`
	args := []any{tblSchema}
	if tblName != "" {
		query += ` AND parent_t.name = @p2`
		args = append(args, tblName)
	}
	query += ` ORDER BY parent_t.name, fk.name, fkc.constraint_column_id`

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
			refSchema                         sql.NullString
			deleteRule, updateRule            sql.NullString
			ordinalPosition                   int64
		)
		if err = rows.Scan(&constraintName, &fkTable, &fkColumn, &ordinalPosition,
			&refSchema, &refTable, &refCol, &deleteRule, &updateRule); err != nil {
			return nil, errw(err)
		}

		k := fkKey{table: fkTable, name: constraintName}
		fk, ok := byKey[k]
		if !ok {
			fk = &metadata.ForeignKey{
				Name:      constraintName,
				Table:     fkTable,
				RefSchema: refSchema.String,
				RefTable:  refTable,
				OnDelete:  deleteRule.String,
				OnUpdate:  updateRule.String,
			}
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, fkColumn)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}
	return fks, nil
}

// getMSSQLIncomingFKs returns the foreign-key constraints declared on
// other tables in tblSchema whose referenced side is tblName. Same
// query shape as getMSSQLForeignKeys, filtered on the *referenced*
// table's schema/name rather than the parent's.
func getMSSQLIncomingFKs(ctx context.Context, db sqlz.DB, tblSchema, tblName string,
) ([]*metadata.ForeignKey, error) {
	log := lg.FromContext(ctx)

	// The referencing-side schema filter (parent_s.name = @p1) keeps
	// the result scoped to FKs whose owning table also lives in
	// tblSchema. Without it, a cross-schema FK (e.g. otherschema.child
	// → tblSchema.tblName) would appear here even though the
	// referencing table isn't part of the schema being inspected,
	// producing an [FK.Incoming] entry pointing at a table that
	// source-level inspect would never include in [Source.Tables].
	const query = `SELECT
  fk.name                                                AS constraint_name,
  parent_t.name                                          AS fk_table,
  parent_c.name                                          AS fk_column,
  fkc.constraint_column_id                               AS ordinal_position,
  ref_t.name                                             AS ref_table,
  ref_c.name                                             AS ref_column,
  REPLACE(fk.delete_referential_action_desc, '_', ' ')   AS delete_rule,
  REPLACE(fk.update_referential_action_desc, '_', ' ')   AS update_rule
FROM sys.foreign_keys        AS fk
JOIN sys.foreign_key_columns AS fkc      ON fkc.constraint_object_id = fk.object_id
JOIN sys.objects             AS parent_t ON parent_t.object_id       = fkc.parent_object_id
JOIN sys.schemas             AS parent_s ON parent_s.schema_id       = parent_t.schema_id
JOIN sys.columns             AS parent_c ON parent_c.object_id       = fkc.parent_object_id
                                        AND parent_c.column_id       = fkc.parent_column_id
JOIN sys.objects             AS ref_t    ON ref_t.object_id          = fkc.referenced_object_id
JOIN sys.schemas             AS ref_s    ON ref_s.schema_id          = ref_t.schema_id
JOIN sys.columns             AS ref_c    ON ref_c.object_id          = fkc.referenced_object_id
                                        AND ref_c.column_id          = fkc.referenced_column_id
WHERE parent_s.name = @p1
  AND ref_s.name    = @p1
  AND ref_t.name    = @p2
ORDER BY parent_t.name, fk.name, fkc.constraint_column_id`

	rows, err := db.QueryContext(ctx, query, tblSchema, tblName)
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
			// RefSchema intentionally left empty: the referenced table
			// is the current one we're inspecting, which lives in
			// tblSchema by construction.
			byKey[k] = fk
			fks = append(fks, fk)
		}
		fk.Columns = append(fk.Columns, fkColumn)
		fk.RefColumns = append(fk.RefColumns, refCol)
	}
	return fks, errw(rows.Err())
}

// getAllTables returns all of the table names, and the table types
// (i.e. "BASE TABLE" or "VIEW") in the given schema. The query is scoped to
// tblSchema because the connected user typically sees tables across multiple
// schemas (at minimum dbo + INFORMATION_SCHEMA); without the filter, tables
// outside the source's current schema leak into source-level inspect, and
// same-named tables across schemas collide on the bare table name. See #613.
func getAllTables(ctx context.Context, db sqlz.DB, tblSchema string) (tblNames, tblTypes []string, err error) {
	log := lg.FromContext(ctx)

	const query = `SELECT TABLE_NAME, TABLE_TYPE FROM INFORMATION_SCHEMA.TABLES
WHERE TABLE_SCHEMA = @p1 AND (TABLE_TYPE = 'BASE TABLE' OR TABLE_TYPE = 'VIEW')
ORDER BY TABLE_NAME ASC, TABLE_TYPE ASC`

	rows, err := db.QueryContext(ctx, query, tblSchema)
	if err != nil {
		return nil, nil, errw(err)
	}
	defer sqlz.CloseRows(log, rows)

	for rows.Next() {
		var tblName, tblType string
		err = rows.Scan(&tblName, &tblType)
		if err != nil {
			return nil, nil, errw(err)
		}
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		tblNames = append(tblNames, tblName)
		tblTypes = append(tblTypes, tblType)
	}

	if rows.Err() != nil {
		return nil, nil, errw(rows.Err())
	}

	return tblNames, tblTypes, nil
}

func getColumnMeta(ctx context.Context, db sqlz.DB, tblCatalog, tblSchema, tblName string) ([]columnMeta, error) {
	log := lg.FromContext(ctx)

	// The query joins INFORMATION_SCHEMA.COLUMNS with sys.* catalog views to
	// detect IDENTITY columns (sys.identity_columns) and computed/generated
	// columns (sys.computed_columns). The join goes through sys.schemas and
	// sys.objects to anchor on object_id without relying on OBJECT_ID() string
	// concatenation. COLLATION_NAME is already in INFORMATION_SCHEMA.COLUMNS.
	const query = `SELECT
		c.TABLE_CATALOG, c.TABLE_SCHEMA, c.TABLE_NAME,
		c.COLUMN_NAME, c.ORDINAL_POSITION, c.COLUMN_DEFAULT, c.IS_NULLABLE, c.DATA_TYPE,
		c.CHARACTER_MAXIMUM_LENGTH, c.CHARACTER_OCTET_LENGTH,
		c.NUMERIC_PRECISION, c.NUMERIC_PRECISION_RADIX, c.NUMERIC_SCALE,
		c.DATETIME_PRECISION,
		c.CHARACTER_SET_CATALOG, c.CHARACTER_SET_SCHEMA, c.CHARACTER_SET_NAME,
		c.COLLATION_CATALOG, c.COLLATION_SCHEMA, c.COLLATION_NAME,
		c.DOMAIN_CATALOG, c.DOMAIN_SCHEMA, c.DOMAIN_NAME,
		CAST(CASE WHEN ic.column_id IS NOT NULL THEN 1 ELSE 0 END AS BIT) AS is_identity,
		CAST(CASE WHEN cc.column_id IS NOT NULL THEN 1 ELSE 0 END AS BIT) AS is_computed,
		cc.definition AS generated_expr
	FROM INFORMATION_SCHEMA.COLUMNS c
	JOIN sys.schemas s ON s.name = c.TABLE_SCHEMA
	JOIN sys.objects o ON o.name = c.TABLE_NAME AND o.schema_id = s.schema_id
	LEFT JOIN sys.identity_columns ic
		ON ic.object_id = o.object_id AND ic.name = c.COLUMN_NAME
	LEFT JOIN sys.computed_columns cc
		ON cc.object_id = o.object_id AND cc.name = c.COLUMN_NAME
	WHERE c.TABLE_CATALOG = @p1 AND c.TABLE_SCHEMA = @p2 AND c.TABLE_NAME = @p3
	ORDER BY c.ORDINAL_POSITION`

	rows, err := db.QueryContext(ctx, query, tblCatalog, tblSchema, tblName)
	if err != nil {
		return nil, errw(err)
	}

	defer sqlz.CloseRows(log, rows)

	var cols []columnMeta

	for rows.Next() {
		c := columnMeta{}
		err = rows.Scan(&c.TableCatalog, &c.TableSchema, &c.TableName, &c.ColumnName, &c.OrdinalPosition,
			&c.ColumnDefault, &c.Nullable, &c.DataType, &c.CharMaxLength, &c.CharOctetLength, &c.NumericPrecision,
			&c.NumericPrecisionRadix, &c.NumericScale, &c.DateTimePrecision, &c.CharSetCatalog, &c.CharSetSchema,
			&c.CharSetName, &c.CollationCatalog, &c.CollationSchema, &c.CollationName, &c.DomainCatalog,
			&c.DomainSchema, &c.DomainName,
			&c.IsIdentity, &c.IsComputed, &c.GeneratedExpr)
		if err != nil {
			return nil, errw(err)
		}
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)
		cols = append(cols, c)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
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
		return nil, errw(err)
	}
	progress.Incr(ctx, 1)
	debugz.DebugSleep(ctx)

	defer sqlz.CloseRows(log, rows)

	var constraints []constraintMeta
	for rows.Next() {
		c := constraintMeta{}
		err = rows.Scan(&c.TableCatalog, &c.TableSchema, &c.TableName, &c.ConstraintType, &c.ColumnName,
			&c.ConstraintName)
		if err != nil {
			return nil, errw(err)
		}
		progress.Incr(ctx, 1)
		debugz.DebugSleep(ctx)

		constraints = append(constraints, c)
	}

	if err = rows.Err(); err != nil {
		return nil, errw(err)
	}

	return constraints, nil
}

// constraintMeta models constraint metadata from information schema.
type constraintMeta struct {
	TableCatalog   string `db:"TABLE_CATALOG"` // REVISIT: why do we have the `db` tag here?
	TableSchema    string `db:"TABLE_SCHEMA"`
	TableName      string `db:"TABLE_NAME"`
	ConstraintType string `db:"CONSTRAINT_TYPE"`
	ColumnName     string `db:"COLUMN_NAME"`
	ConstraintName string `db:"CONSTRAINT_NAME"`
}

// columnMeta models column metadata from information schema.
type columnMeta struct { //nolint:govet // field alignment
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
	// IsIdentity is true when the column is declared with IDENTITY.
	IsIdentity bool `db:"is_identity"`
	// IsComputed is true when the column is a computed (generated) column.
	IsComputed bool `db:"is_computed"`
	// GeneratedExpr is the T-SQL expression for computed columns; empty otherwise.
	GeneratedExpr sql.NullString `db:"generated_expr"`
}

// getMSSQLCheckConstraints returns the CHECK constraints declared on tables in
// the given schema. If tblName is empty, constraints for every table in the
// schema are returned; otherwise only constraints on tblName are returned.
// The Clause field holds the engine-formatted expression as stored in
// sys.check_constraints.definition.
func getMSSQLCheckConstraints(ctx context.Context, db sqlz.DB, tblSchema, tblName string,
) ([]*metadata.CheckConstraint, error) {
	log := lg.FromContext(ctx)

	query := `SELECT
  t.name        AS table_name,
  cc.name       AS constraint_name,
  cc.definition AS clause
FROM sys.check_constraints cc
JOIN sys.objects t ON t.object_id = cc.parent_object_id
JOIN sys.schemas s ON s.schema_id = t.schema_id
WHERE s.name = @p1`
	args := []any{tblSchema}
	if tblName != "" {
		query += ` AND t.name = @p2`
		args = append(args, tblName)
	}
	query += ` ORDER BY t.name, cc.name`

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

// getMSSQLTriggers returns the DML triggers (parent_class = 1) attached to
// user tables in the given schema. If tblName is empty, triggers for every
// table in the schema are returned; otherwise only triggers on tblName are
// returned.
//
// Timing is derived from is_instead_of_trigger: "INSTEAD OF" or "AFTER".
// SQL Server has no BEFORE trigger timing. Events are built from the
// ExecIsInsertTrigger / ExecIsUpdateTrigger / ExecIsDeleteTrigger properties
// in INSERT, UPDATE, DELETE order. Enabled = NOT is_disabled as a *bool.
// Definition comes from sys.sql_modules.definition (not truncated).
func getMSSQLTriggers(ctx context.Context, db sqlz.DB, tblSchema, tblName string,
) ([]*metadata.Trigger, error) {
	log := lg.FromContext(ctx)

	query := `SELECT
  OBJECT_NAME(tr.parent_id)                                           AS table_name,
  tr.name                                                              AS trigger_name,
  CAST(CASE WHEN tr.is_instead_of_trigger = 1
            THEN 'INSTEAD OF' ELSE 'AFTER' END AS NVARCHAR(20))        AS timing,
  CAST(OBJECTPROPERTY(tr.object_id, 'ExecIsInsertTrigger') AS BIT)    AS on_insert,
  CAST(OBJECTPROPERTY(tr.object_id, 'ExecIsUpdateTrigger') AS BIT)    AS on_update,
  CAST(OBJECTPROPERTY(tr.object_id, 'ExecIsDeleteTrigger') AS BIT)    AS on_delete,
  CAST(CASE WHEN tr.is_disabled = 0 THEN 1 ELSE 0 END AS BIT)        AS enabled,
  sm.definition                                                         AS definition
FROM sys.triggers    tr
JOIN sys.objects     t  ON t.object_id  = tr.parent_id
JOIN sys.schemas     s  ON s.schema_id  = t.schema_id
JOIN sys.sql_modules sm ON sm.object_id = tr.object_id
WHERE tr.parent_class = 1
  AND s.name = @p1`
	args := []any{tblSchema}
	if tblName != "" {
		query += ` AND t.name = @p2`
		args = append(args, tblName)
	}
	query += ` ORDER BY t.name, tr.name`

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
			onUpdate   bool
			onDelete   bool
			enabled    bool
			definition string
		)
		if err = rows.Scan(&tblNameVal, &trigName, &timing,
			&onInsert, &onUpdate, &onDelete, &enabled, &definition); err != nil {
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

// getMSSQLViewDefinitions returns a map of view name → defining SQL for
// views in the given schema. If tblName is non-empty, only that view is
// returned; passing an empty tblName returns all views. The definition is
// sourced from sys.sql_modules.definition, which does not truncate at
// 4000 chars as INFORMATION_SCHEMA.VIEWS.VIEW_DEFINITION does.
func getMSSQLViewDefinitions(ctx context.Context, db sqlz.DB, tblSchema, tblName string,
) (map[string]string, error) {
	log := lg.FromContext(ctx)

	query := `SELECT
  v.name        AS view_name,
  sm.definition AS definition
FROM sys.views       v
JOIN sys.schemas     s  ON s.schema_id  = v.schema_id
JOIN sys.sql_modules sm ON sm.object_id = v.object_id
WHERE s.name = @p1`
	args := []any{tblSchema}
	if tblName != "" {
		query += ` AND v.name = @p2`
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

// semverRx matches a leading dotted-numeric version token (up to three parts).
var semverRx = regexp.MustCompile(`^v?(\d+(?:\.\d+){0,2})`)

// parseSemver normalizes a SQL Server ProductVersion string to canonical semver
// (e.g. "v16.0.4115"). ProductVersion is four-part (major.minor.build.revision);
// the regex caps it at the first three parts.
func parseSemver(raw string) (string, error) {
	m := semverRx.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return "", errz.Errorf("no semver in sqlserver version string: %q", raw)
	}
	v := semver.Canonical("v" + m[1])
	if !semver.IsValid(v) {
		return "", errz.Errorf("invalid sqlserver semver %q from %q", v, raw)
	}
	return v, nil
}

// DBSemver implements driver.SQLDriver.
func (d *driveri) DBSemver(ctx context.Context, db sqlz.DB) (string, error) {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT SERVERPROPERTY('ProductVersion')").Scan(&raw); err != nil {
		return "", errw(err)
	}
	return parseSemver(raw)
}
